import { json, error } from '@sveltejs/kit';
import { getGithubAccount } from '$lib/server/models/userModel.js';
import { listBranches } from '$lib/server/github.js';

export const GET = async ({ url, locals }) => {
	if (!locals.user) {
		throw error(401, 'Unauthorized');
	}
	const account = await getGithubAccount(locals.user.id);
	if (!account) {
		return json({ branches: [] });
	}
	const owner = url.searchParams.get('owner');
	const repo = url.searchParams.get('repo');
	if (!owner || !repo) {
		throw error(422, 'Missing owner or repo');
	}
	const branches = await listBranches(account.access_token, owner, repo);
	return json({ branches });
};
