import { json, error } from '@sveltejs/kit';
import { findDeploymentById, listVersions } from '$lib/server/models/deploymentModel.js';
import { createTask } from '$lib/server/models/taskModel.js';

export const POST = async ({ params, locals, request }) => {
  if (!locals.user) {
    throw error(401, 'Unauthorized');
  }
  const deployment = await findDeploymentById(params.id);
  if (!deployment) {
    throw error(404, 'Deployment not found');
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
  await createTask('rollback', { deploymentId: deployment.id, version: target });
  return json({ ok: true });
};
