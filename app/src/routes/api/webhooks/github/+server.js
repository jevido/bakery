import { json } from '@sveltejs/kit';
import { Buffer } from 'node:buffer';
import { createHmac, timingSafeEqual } from 'node:crypto';
import { getConfig } from '$lib/server/config.js';
import { createLogger } from '$lib/server/logger.js';
import { startSelfUpdate } from '$lib/server/selfUpdater.js';
import { listDeploymentsByRepository } from '$lib/server/models/deploymentModel.js';
import { createTask, hasActiveDeployTask } from '$lib/server/models/taskModel.js';

const logger = createLogger('github-webhook');

function verifySignature(secret, payloadBuffer, signatureHeader) {
	if (!secret) return false;
	if (!signatureHeader || !signatureHeader.startsWith('sha256=')) {
		return false;
	}
	const expected = `sha256=${createHmac('sha256', secret).update(payloadBuffer).digest('hex')}`;
	const providedBuffer = Buffer.from(signatureHeader);
	const expectedBuffer = Buffer.from(expected);
	if (providedBuffer.length !== expectedBuffer.length) {
		return false;
	}
	return timingSafeEqual(providedBuffer, expectedBuffer);
}

function normalizeBranch(value = '') {
	return value.replace(/^refs\/heads\//, '');
}

function matchesSelfUpdate(repo, branch, config) {
	if (!repo) return false;
	const expectedBranch = normalizeBranch(config.selfUpdateBranch);
	return repo === config.selfUpdateRepository && branch === expectedBranch;
}

async function handleSelfUpdate({ payload, delivery, repo, branch }) {
	const meta = {
		source: 'github-webhook',
		delivery,
		repo,
		ref: payload?.ref,
		branch,
		after: payload?.after,
		before: payload?.before
	};

	try {
		startSelfUpdate(meta);
	} catch (error) {
		if (error?.code === 'UPDATE_IN_PROGRESS') {
			logger.info('Webhook ignored because an update is already running', {
				delivery
			});
			return json({ ok: true, accepted: false, reason: 'update_in_progress' }, { status: 202 });
		}
		logger.error('Failed to start Bakery self-update from webhook', {
			delivery,
			error: error?.message || String(error)
		});
		return json({ ok: false, error: 'update_failed' }, { status: 500 });
	}

	return json({ ok: true, accepted: true, scope: 'self-update' });
}

async function handleDeploymentPush({ repo, branch, payload, delivery }) {
	const deployments = await listDeploymentsByRepository(repo, branch);
	if (!deployments.length) {
		logger.info('No deployments matched webhook push', { delivery, repo, branch });
		return json({ ok: true, ignored: true, reason: 'no_matching_deployments' });
	}

	const queued = [];
	for (const deployment of deployments) {
		if (await hasActiveDeployTask(deployment.id)) {
			logger.info('Skipping auto-deploy; existing task in progress', {
				delivery,
				deploymentId: deployment.id
			});
			continue;
		}
		await createTask(
			'deploy',
			{ deploymentId: deployment.id, commitSha: payload?.after || null },
			{ nodeId: deployment.node_id }
		);
		queued.push(deployment.id);
		logger.info('Queued deployment from GitHub webhook', {
			delivery,
			deploymentId: deployment.id,
			repo,
			branch
		});
	}

	if (!queued.length) {
		return json({ ok: true, accepted: false, reason: 'already_in_progress' }, { status: 202 });
	}

	return json({ ok: true, accepted: true, deployments: queued });
}

export const POST = async ({ request }) => {
	const config = getConfig();
	if (!config.githubAppWebhookSecret) {
		return json(
			{ ok: false, error: 'webhook_not_configured' },
			{ status: 503 }
		);
	}

	const signature = request.headers.get('x-hub-signature-256');
	const event = request.headers.get('x-github-event') || 'unknown';
	const delivery = request.headers.get('x-github-delivery') || undefined;
	const payloadBuffer = Buffer.from(await request.arrayBuffer());

	if (!verifySignature(config.githubAppWebhookSecret, payloadBuffer, signature)) {
		logger.warn('Rejected GitHub webhook with invalid signature', { delivery });
		return json({ ok: false, error: 'invalid_signature' }, { status: 401 });
	}

	let payload;
	try {
		payload = payloadBuffer.length ? JSON.parse(payloadBuffer.toString('utf8')) : {};
	} catch (error) {
		logger.warn('Failed to parse GitHub webhook payload', {
			delivery,
			error: error.message
		});
		return json({ ok: false, error: 'invalid_payload' }, { status: 400 });
	}

	if (event === 'ping') {
		return json({ ok: true, pong: true });
	}

	if (event !== 'push') {
		logger.info('Ignoring GitHub webhook event', { delivery, event });
		return json({ ok: true, ignored: true, reason: 'unsupported_event' });
	}

	const ref = payload?.ref || '';
	const branch = normalizeBranch(ref);
	const repo = payload?.repository?.full_name;
	if (!repo) {
		logger.warn('Received push event without repository metadata', { delivery });
		return json({ ok: false, error: 'invalid_event' }, { status: 400 });
	}

	if (matchesSelfUpdate(repo, branch, config)) {
		return handleSelfUpdate({ payload, delivery, repo, branch });
	}

	return handleDeploymentPush({ repo, branch, payload, delivery });
};
