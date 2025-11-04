import { json, error } from '@sveltejs/kit';
import { findDeploymentById, recordDeploymentLog } from '$lib/server/models/deploymentModel.js';
import { createTask } from '$lib/server/models/taskModel.js';

export const POST = async ({ params, locals }) => {
  if (!locals.user) {
    throw error(401, 'Unauthorized');
  }
  const deployment = await findDeploymentById(params.id);
  if (!deployment) {
    throw error(404, 'Deployment not found');
  }
  await createTask('deploy', { deploymentId: deployment.id });
  await recordDeploymentLog(deployment.id, 'info', 'Deployment manually triggered', {
    userId: locals.user.id
  });
  return json({ ok: true });
};
