import { json, error } from '@sveltejs/kit';
import { z } from 'zod';
import { updateNode, findNodeById } from '$lib/server/models/nodeModel.js';

const schema = z.object({
	name: z.string().min(3).max(64)
});

export const PATCH = async ({ params, request, locals }) => {
	if (!locals.user) {
		throw error(401, 'Unauthorized');
	}
	const node = await findNodeById(params.id);
	if (!node || node.owner_id !== locals.user.id) {
		throw error(404, 'Node not found');
	}
	const body = await request.json().catch(() => ({}));
	const parsed = schema.safeParse(body);
	if (!parsed.success) {
		throw error(422, 'Invalid payload');
	}
	const updated = await updateNode(node.id, { name: parsed.data.name });
	return json({ node: updated });
};
