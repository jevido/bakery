import { json, error } from '@sveltejs/kit';
import { z } from 'zod';
import { consumeInstallToken } from '$lib/server/models/nodeModel.js';

const schema = z.object({
	token: z.string().min(10),
	hostname: z.string().optional(),
	platform: z.string().optional(),
	arch: z.string().optional(),
	version: z.string().optional()
});

export const POST = async ({ request }) => {
	const body = await request.json().catch(() => ({}));
	const parsed = schema.safeParse(body);
	if (!parsed.success) {
		throw error(422, 'Invalid registration payload');
	}
	const metadata = Object.fromEntries(
		Object.entries(parsed.data).filter(([key]) => key !== 'token' && parsed.data[key] != null)
	);
	const result = await consumeInstallToken(parsed.data.token, metadata);
	if (!result) {
		throw error(400, 'Invalid or expired token');
	}
	return json({
		nodeId: result.node.id,
		pairingCode: result.pairingCode,
		apiToken: result.apiToken
	});
};
