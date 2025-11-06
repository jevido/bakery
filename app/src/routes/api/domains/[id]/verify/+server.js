import { json, error } from '@sveltejs/kit';
import { findDomainById, updateDomain } from '$lib/server/models/domainModel.js';
import { requestCertificate } from '$lib/server/certbot.js';
import { listDomains } from '$lib/server/models/domainModel.js';
import { findDeploymentById } from '$lib/server/models/deploymentModel.js';
import { writeDeploymentConfig, reloadNginx } from '$lib/server/nginx.js';

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
