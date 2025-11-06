import { json, error } from '@sveltejs/kit';
import { z } from 'zod';
import { findNodeById, markNodeActive } from '$lib/server/models/nodeModel.js';

const schema = z.object({
	code: z.string().min(4)
});

function sanitizeNode(node) {
	if (!node) return node;
	const {
		api_token: apiToken,
		install_token: installToken,
		pairing_code: pairingCode,
		...rest
	} = node;
	return {
		...rest,
		has_install_token: Boolean(installToken),
		pairing_pending: Boolean(pairingCode)
	};
}

export const POST = async ({ params, locals, request }) => {
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
		throw error(422, 'Validation failed');
	}
	const normalizedCode = parsed.data.code.trim().toUpperCase();
	const updated = await markNodeActive(node.id, normalizedCode);
	if (!updated) {
		throw error(400, 'Invalid pairing code');
	}
	return json({ node: sanitizeNode(updated) });
};
