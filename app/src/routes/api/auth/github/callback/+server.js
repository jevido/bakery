import { json, error } from '@sveltejs/kit';
import { exchangeCodeForToken, createOctokitFromToken } from '$lib/server/github.js';
import { consumeOAuthState } from '$lib/server/models/oauthModel.js';
import { updateUserGithubLink } from '$lib/server/models/userModel.js';
import { log } from '$lib/server/logger.js';

export const GET = async ({ url }) => {
	const code = url.searchParams.get('code');
	const state = url.searchParams.get('state');
	if (!code || !state) {
		throw error(422, 'Missing code or state');
	}
	const record = await consumeOAuthState(state, 'github');
	if (!record) {
		throw error(400, 'Invalid OAuth state');
	}
	try {
		const { accessToken, encryptedToken } = await exchangeCodeForToken(code);
		const octokit = createOctokitFromToken(accessToken);
		const { data } = await octokit.users.getAuthenticated();
		await updateUserGithubLink(record.user_id, data.id, encryptedToken);
		await log('info', 'GitHub account linked', { userId: record.user_id });
		return json({ ok: true });
	} catch (err) {
		await log('error', 'GitHub OAuth failed', { error: err.message });
		throw error(500, 'GitHub link failed');
	}
};
