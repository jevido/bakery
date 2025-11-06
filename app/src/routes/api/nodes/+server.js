import { json, error } from '@sveltejs/kit';
import { z } from 'zod';
import { listNodes, createNode, deleteNode } from '$lib/server/models/nodeModel.js';
import { getConfig } from '$lib/server/config.js';

const createNodeSchema = z.object({
	name: z.string().min(3)
});

function sanitizeNode(node) {
	if (!node) return node;
	const {
		api_token: apiToken,
		install_token: installToken,
		pairing_code: pairingCode,
		...rest
	} = node;
	return {
		...rest,
		has_install_token: Boolean(installToken),
		pairing_pending: Boolean(pairingCode)
	};
}

function getInstallContext(installToken = null) {
	const config = getConfig();
	const apiBase = config.baseUrl;
	let command = null;
	let warning = null;
	let blocked = false;
	let parsedUrl;
	try {
		parsedUrl = new URL(apiBase);
	} catch (error) {
		warning = `Bakery base URL \`${apiBase}\` is not valid. Set BAKERY_BASE_URL to a reachable HTTPS endpoint.`;
		blocked = true;
		return { command, warning, apiBase, blocked };
	}

	const hostname = parsedUrl.hostname.toLowerCase();
	const isLoopback =
		hostname === 'localhost' ||
		hostname === '127.0.0.1' ||
		hostname === '::1' ||
		hostname.endsWith('.local');

	if (isLoopback) {
		warning = `The Bakery base URL is set to \`${apiBase}\`. Remote servers cannot reach localhost addresses. Update BAKERY_BASE_URL to a public IP or domain before installing the agent.`;
		blocked = true;
		return { command, warning, apiBase: parsedUrl.toString(), blocked };
	}

	const scriptUrl = new URL('/api/agent/install.sh', parsedUrl).toString();
	if (installToken) {
		command = `curl -fsSL ${scriptUrl} | sudo bash -s -- --token ${installToken} --api ${parsedUrl.toString()}`;
	}
	return { command, warning, apiBase: parsedUrl.toString(), blocked };
}

export const GET = async ({ locals }) => {
	if (!locals.user) {
		throw error(401, 'Unauthorized');
	}
	const nodes = await listNodes(locals.user.id);
	const install = getInstallContext();
	return json({
		controlPlane: {
			id: 'control-plane',
			name: 'Control plane',
			status: 'active'
		},
		nodes: nodes.map(sanitizeNode),
		installBlocked: install.blocked,
		installWarning: install.warning,
		apiBase: install.apiBase
	});
};

export const POST = async ({ request, locals }) => {
	if (!locals.user) {
		throw error(401, 'Unauthorized');
	}
	const body = await request.json().catch(() => ({}));
	const parsed = createNodeSchema.safeParse(body);
	if (!parsed.success) {
		throw error(422, 'Validation failed');
	}
	const { node, installToken } = await createNode({
		ownerId: locals.user.id,
		name: parsed.data.name
	});
	const install = getInstallContext(installToken);
	return json({
		node: sanitizeNode(node),
		installToken,
		installCommand: install.command,
		installCommandWarning: install.warning,
		apiBase: install.apiBase
	});
};

export const DELETE = async ({ url, locals }) => {
	if (!locals.user) {
		throw error(401, 'Unauthorized');
	}
	const id = url.searchParams.get('id');
	if (!id) {
		throw error(422, 'id required');
	}
	await deleteNode(id, locals.user.id);
	return json({ ok: true });
};
