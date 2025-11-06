import { json, error } from '@sveltejs/kit';
import { z } from 'zod';
import { requireAgent } from '$lib/server/agentAuth.js';
import { findTaskById, finishTask } from '$lib/server/models/taskModel.js';
import { deleteDeployment } from '$lib/server/models/deploymentModel.js';

const schema = z.object({
	status: z.enum(['completed', 'failed']),
	error: z.string().optional()
});

export const POST = async ({ params, request }) => {
	const node = await requireAgent(request);
	const task = await findTaskById(params.id);
	if (!task || task.node_id !== node.id) {
		throw error(404, 'Task not found');
	}
	const body = await request.json().catch(() => ({}));
	const parsed = schema.safeParse(body);
	if (!parsed.success) {
		throw error(422, 'Invalid payload');
	}
	const { status, error: message } = parsed.data;
	await finishTask(
		task.id,
		status,
		status === 'failed' ? (message ?? 'Agent reported error') : null
	);

	if (status === 'completed') {
		if (task.type === 'cleanup' && task.payload?.deploymentId) {
			await deleteDeployment(task.payload.deploymentId).catch(() => {});
		}
	}
	return json({ ok: true });
};
