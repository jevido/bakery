import { json, error } from '@sveltejs/kit';
import { requireAgent } from '$lib/server/agentAuth.js';
import { findDeploymentById } from '$lib/server/models/deploymentModel.js';
import { exportEnvVars } from '$lib/server/models/envModel.js';
import { listDomains } from '$lib/server/models/domainModel.js';
import { getGithubAccount } from '$lib/server/models/userModel.js';
import { decrypt } from '$lib/server/crypto.js';

export const GET = async ({ params, request }) => {
	const node = await requireAgent(request);
	const deployment = await findDeploymentById(params.id);
	if (!deployment || deployment.node_id !== node.id) {
		throw error(404, 'Deployment not found');
	}
	const [envVars, domains] = await Promise.all([
		exportEnvVars(deployment.id),
		listDomains(deployment.id)
	]);
	const account = await getGithubAccount(deployment.owner_id);
	return json({
		deployment: {
			id: deployment.id,
			name: deployment.name,
			repository: deployment.repository,
			branch: deployment.branch,
			blue_green_enabled: deployment.blue_green_enabled,
			active_slot: deployment.active_slot,
			dockerized: deployment.dockerized,
			dockerfile_path: deployment.dockerfile_path,
			build_context: deployment.build_context
		},
		environment: envVars,
		domains,
		githubAccessToken: account ? decrypt(account.access_token) : null
	});
};
