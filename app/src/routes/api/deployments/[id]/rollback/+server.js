import { json, error } from '@sveltejs/kit';
import { findDeploymentById, listVersions } from '$lib/server/models/deploymentModel.js';
import { createTask } from '$lib/server/models/taskModel.js';
import { findNodeById } from '$lib/server/models/nodeModel.js';

export const POST = async ({ params, locals, request }) => {
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
	if (deployment.node_id) {
		const node = await findNodeById(deployment.node_id);
		if (!node || (node.owner_id !== locals.user.id && !locals.user.is_admin)) {
			throw error(400, 'Deployment node is unavailable');
		}
		if (node.status !== 'active') {
			throw error(400, 'Node is not active');
		}
	}
	const body = await request.json().catch(() => ({}));
	const versionId = body.versionId;
	if (!versionId) {
		throw error(422, 'versionId required');
	}
	const versions = await listVersions(deployment.id);
	const target = versions.find((v) => v.id === versionId);
	if (!target) {
		throw error(404, 'Version not found');
	}
	await createTask(
		'rollback',
		{ deploymentId: deployment.id, version: target },
		{ nodeId: deployment.node_id }
	);
	return json({ ok: true });
};
