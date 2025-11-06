import { json, error } from '@sveltejs/kit';
import { z } from 'zod';
import { requireAgent } from '$lib/server/agentAuth.js';
import { findDeploymentById, recordDeploymentLog } from '$lib/server/models/deploymentModel.js';

const schema = z.object({
	level: z.enum(['info', 'warn', 'error']),
	message: z.string().min(1),
	meta: z.record(z.any()).optional()
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
	await recordDeploymentLog(
		deployment.id,
		parsed.data.level,
		parsed.data.message,
		parsed.data.meta
	);
	return json({ ok: true });
};
