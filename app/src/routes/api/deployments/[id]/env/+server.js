import { json, error } from '@sveltejs/kit';
import { findDeploymentById } from '$lib/server/models/deploymentModel.js';
import { upsertEnvVar, removeEnvVar } from '$lib/server/models/envModel.js';

export const POST = async ({ params, locals, request }) => {
  if (!locals.user) {
    throw error(401, 'Unauthorized');
  }
  const deployment = await findDeploymentById(params.id);
  if (!deployment) {
    throw error(404, 'Deployment not found');
  }
  const body = await request.json().catch(() => ({}));
  const { key, value } = body;
  if (!key || typeof value !== 'string') {
    throw error(422, 'Invalid payload');
  }
  await upsertEnvVar(deployment.id, key, value);
  return json({ ok: true });
};

export const DELETE = async ({ params, locals, request }) => {
  if (!locals.user) {
    throw error(401, 'Unauthorized');
  }
  const deployment = await findDeploymentById(params.id);
  if (!deployment) {
    throw error(404, 'Deployment not found');
  }
  const body = await request.json().catch(() => ({}));
  const { key } = body;
  if (!key) {
    throw error(422, 'Key required');
  }
  await removeEnvVar(deployment.id, key);
  return json({ ok: true });
};
