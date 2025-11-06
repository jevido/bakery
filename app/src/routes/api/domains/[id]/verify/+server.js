import { json, error } from '@sveltejs/kit';
import { findDomainById, updateDomain, listDomains } from '$lib/server/models/domainModel.js';
import { requestCertificate } from '$lib/server/certbot.js';
import { findDeploymentById } from '$lib/server/models/deploymentModel.js';
import { writeDeploymentConfig, reloadNginx } from '$lib/server/nginx.js';
import { createTask } from '$lib/server/models/taskModel.js';
import { findNodeById } from '$lib/server/models/nodeModel.js';

export const POST = async ({ params, locals }) => {
	if (!locals.user) {
		throw error(401, 'Unauthorized');
	}
	const domain = await findDomainById(params.id);
	if (!domain) {
		throw error(404, 'Domain not found');
	}
	const deployment = await findDeploymentById(domain.deployment_id);
	if (!deployment || (deployment.owner_id && deployment.owner_id !== locals.user.id)) {
		throw error(403, 'Forbidden');
	}

	if (deployment.node_id) {
		const node = await findNodeById(deployment.node_id);
		if (!node || (node.owner_id !== locals.user.id && !locals.user.is_admin)) {
			throw error(400, 'Deployment node is unavailable');
		}
		if (node.status !== 'active') {
			throw error(400, 'Node is not active');
		}
		await updateDomain(domain.id, { verified: true, ssl_status: 'requested' });
		await createTask('deploy', { deploymentId: deployment.id }, { nodeId: deployment.node_id });
		return json({ ok: true, queued: true });
	}

	await requestCertificate([domain.hostname]);
	await updateDomain(domain.id, { verified: true, ssl_status: 'requested' });
	const domains = await listDomains(deployment.id);
	await writeDeploymentConfig({
		deployment,
		domains,
		port: deployment.port || 0,
		slot: deployment.active_slot || 'blue'
	});
	await reloadNginx();
	return json({ ok: true });
};
