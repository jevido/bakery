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
  await createTask('restart', { deploymentId: deployment.id });
  return json({ ok: true });
};
