import { json, error } from '@sveltejs/kit';
import { findDeploymentById } from '$lib/server/models/deploymentModel.js';
import { addDomain, deleteDomain } from '$lib/server/models/domainModel.js';
import { getLocalResolutionHint, isLocalHostname } from '$lib/server/domainUtils.js';

export const POST = async ({ params, locals, request }) => {
	if (!locals.user) {
		throw error(401, 'Unauthorized');
	}
	const deployment = await findDeploymentById(params.id);
	if (!deployment) {
		throw error(404, 'Deployment not found');
	}
	const body = await request.json().catch(() => ({}));
	const { hostname } = body;
	if (!hostname) {
		throw error(422, 'Hostname required');
	}
	if (isLocalHostname(hostname)) {
		throw error(
			400,
			'Local-only hostnames (like *.local or private IPs) are disabled for now. Public DNS support is available today and local overrides will return in a future update.'
		);
	}
	const domain = await addDomain({ deploymentId: deployment.id, hostname });
	const hint = getLocalResolutionHint(domain.hostname);
	return json({ domain: { ...domain, resolution_hint: hint } }, 201);
};

export const DELETE = async ({ locals, request }) => {
	if (!locals.user) {
		throw error(401, 'Unauthorized');
	}
	const body = await request.json().catch(() => ({}));
	const { domainId } = body;
	if (!domainId) {
		throw error(422, 'domainId required');
	}
	await deleteDomain(domainId);
	return json({ ok: true });
};
