import { json, error } from '@sveltejs/kit';
import { findDeploymentById } from '$lib/server/models/deploymentModel.js';
import { addDomain, deleteDomain } from '$lib/server/models/domainModel.js';

export const POST = async ({ params, locals, request }) => {
  if (!locals.user) {
    throw error(401, 'Unauthorized');
  }
  const deployment = await findDeploymentById(params.id);
  if (!deployment) {
    throw error(404, 'Deployment not found');
  }
  const body = await request.json().catch(() => ({}));
  const { hostname } = body;
  if (!hostname) {
    throw error(422, 'Hostname required');
  }
  const domain = await addDomain({ deploymentId: deployment.id, hostname });
  return json({ domain }, 201);
};

export const DELETE = async ({ locals, request }) => {
  if (!locals.user) {
    throw error(401, 'Unauthorized');
  }
  const body = await request.json().catch(() => ({}));
  const { domainId } = body;
  if (!domainId) {
    throw error(422, 'domainId required');
  }
  await deleteDomain(domainId);
  return json({ ok: true });
};
