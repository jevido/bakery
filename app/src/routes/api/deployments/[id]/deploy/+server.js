import { json, error } from '@sveltejs/kit';
import { findDeploymentById, recordDeploymentLog } from '$lib/server/models/deploymentModel.js';
import { createTask } from '$lib/server/models/taskModel.js';
import { getGithubAccount } from '$lib/server/models/userModel.js';
import { findNodeById } from '$lib/server/models/nodeModel.js';

export const POST = async ({ params, locals }) => {
	if (!locals.user) {
		throw error(401, 'Unauthorized');
	}
	const deployment = await findDeploymentById(params.id);
	if (!deployment) {
		throw error(404, 'Deployment not found');
	}
	if (deployment.owner_id !== locals.user.id && !locals.user.is_admin) {
		throw error(403, 'Forbidden');
	}
	const ownerAccount = await getGithubAccount(deployment.owner_id);
	if (!ownerAccount) {
		throw error(400, 'Link a GitHub account before deploying this app');
	}

	if (deployment.node_id) {
		const node = await findNodeById(deployment.node_id);
		if (!node || (node.owner_id !== locals.user.id && !locals.user.is_admin)) {
			throw error(400, 'Deployment node is unavailable');
		}
		if (node.status !== 'active') {
			throw error(400, 'Node is not active');
		}
	}
	await createTask('deploy', { deploymentId: deployment.id }, { nodeId: deployment.node_id });
await recordDeploymentLog(deployment.id, 'info', 'Deployment manually triggered', {
	stream: 'system',
	userId: locals.user.id
});
	return json({ ok: true });
};
