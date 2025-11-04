import { getGithubAccount } from '../models/userModel.js';
import { listRepositories, listBranches } from '../lib/github.js';

export const GithubController = {
  async repositories(ctx) {
    const account = await getGithubAccount(ctx.user.id);
    if (!account) {
      return ctx.json({ repositories: [] });
    }
    const repos = await listRepositories(account.access_token);
    return ctx.json({ repositories: repos });
  },

  async branches(ctx) {
    const account = await getGithubAccount(ctx.user.id);
    if (!account) {
      return ctx.json({ branches: [] });
    }
    const { owner, repo } = ctx.url.searchParams;
    if (!owner || !repo) {
      return ctx.json({ error: 'Missing owner or repo' }, 422);
    }
    const branches = await listBranches(account.access_token, owner, repo);
    return ctx.json({ branches });
  }
};
