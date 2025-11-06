import { json } from '@sveltejs/kit';
import { getGithubAccount } from '$lib/server/models/userModel.js';

export const GET = async ({ locals }) => {
	if (!locals.user) {
		return json({ user: null });
	}
	const github = await getGithubAccount(locals.user.id);
	return json({
		user: {
			id: locals.user.id,
			email: locals.user.email,
			is_admin: locals.user.is_admin,
			github_connected: Boolean(github)
		}
	});
};
