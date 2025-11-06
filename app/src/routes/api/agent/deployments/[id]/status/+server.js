import { json, error } from '@sveltejs/kit';
import { z } from 'zod';
import { requireAgent } from '$lib/server/agentAuth.js';
import { findDeploymentById, updateDeployment } from '$lib/server/models/deploymentModel.js';
import { touchNode } from '$lib/server/models/nodeModel.js';

const schema = z.object({
	status: z.string().optional(),
	active_slot: z.enum(['blue', 'green']).optional(),
	dockerized: z.boolean().optional()
});

export const POST = async ({ params, request }) => {
	const node = await requireAgent(request);
	const deployment = await findDeploymentById(params.id);
	if (!deployment || deployment.node_id !== node.id) {
		throw error(404, 'Deployment not found');
	}
	const body = await request.json().catch(() => ({}));
	const parsed = schema.safeParse(body);
	if (!parsed.success) {
		throw error(422, 'Invalid payload');
	}
	const updates = Object.fromEntries(
		Object.entries(parsed.data).filter(([, value]) => value !== undefined)
	);
	if (Object.keys(updates).length === 0) {
		return json({ deployment: deployment });
	}
	const updated = await updateDeployment(deployment.id, updates);
	await touchNode(node.id, { lastTask: 'status-update' });
	return json({ deployment: updated });
};
