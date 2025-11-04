import { json, error } from '@sveltejs/kit';
import { getGithubAccount } from '$lib/server/models/userModel.js';
import { listRepositories } from '$lib/server/github.js';

export const GET = async ({ locals }) => {
  if (!locals.user) {
    throw error(401, 'Unauthorized');
  }
  const account = await getGithubAccount(locals.user.id);
  if (!account) {
    return json({ repositories: [] });
  }
  const repos = await listRepositories(account.access_token);
  return json({ repositories: repos });
};
