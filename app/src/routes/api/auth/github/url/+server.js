import { json, error } from '@sveltejs/kit';
import { getOAuthUrl } from '$lib/server/github.js';
import { createOAuthState } from '$lib/server/models/oauthModel.js';

export const GET = async ({ locals }) => {
	if (!locals.user) {
		throw error(401, 'Unauthorized');
	}
	const { url, state } = getOAuthUrl();
	await createOAuthState(locals.user.id, 'github', state);
	return json({ url, state });
};
