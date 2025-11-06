import { json } from '@sveltejs/kit';
import { z } from 'zod';
import { requireAgent } from '$lib/server/agentAuth.js';
import { touchNode } from '$lib/server/models/nodeModel.js';

const schema = z.object({
	metadata: z.record(z.any()).optional()
});

export const POST = async ({ request }) => {
	const node = await requireAgent(request, { allowInactive: true });
	const body = await request.json().catch(() => ({}));
	const parsed = schema.safeParse(body);
	const metadata = parsed.success && parsed.data.metadata ? parsed.data.metadata : {};
	await touchNode(node.id, metadata);
	return json({ ok: true });
};
