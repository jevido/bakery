import { json } from '@sveltejs/kit';
import { reservePendingTask } from '$lib/server/models/taskModel.js';
import { requireAgent } from '$lib/server/agentAuth.js';

export const POST = async ({ request }) => {
	const node = await requireAgent(request);
	const task = await reservePendingTask(node.id, node.id);
	if (!task) {
		return json({ task: null });
	}
	if (task.node_id && task.node_id !== node.id) {
		return json({ task: null });
	}
	return json({
		task: {
			id: task.id,
			type: task.type,
			payload: task.payload
		}
	});
};
