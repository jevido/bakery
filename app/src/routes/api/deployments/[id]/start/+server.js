import { json, error } from '@sveltejs/kit';
import { findDeploymentById } from '$lib/server/models/deploymentModel.js';
import { createTask } from '$lib/server/models/taskModel.js';

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
	if (deployment.status !== 'inactive') {
		throw error(400, 'Start is only available once the deployment has been stopped.');
	}
	if (!deployment.active_slot) {
		throw error(400, 'Deploy this app at least once before starting it again.');
	}
	await createTask('start', { deploymentId: deployment.id }, { nodeId: deployment.node_id });
	return json({ ok: true });
};
