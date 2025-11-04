import { Octokit } from '@octokit/rest';
import { nanoid } from 'nanoid';
import { getConfig } from './config.js';
import { encrypt, decrypt } from './crypto.js';
import { log } from './logger.js';

const OAUTH_URL = 'https://github.com/login/oauth/authorize';
const TOKEN_URL = 'https://github.com/login/oauth/access_token';

export function getOAuthUrl() {
  const config = getConfig();
  const state = nanoid();
  const url = new URL(OAUTH_URL);
  url.searchParams.set('client_id', config.githubClientId);
  url.searchParams.set('scope', 'repo admin:repo_hook read:user');
  url.searchParams.set('redirect_uri', `${config.baseUrl}/auth/github/callback`);
  url.searchParams.set('state', state);
  return { url: url.toString(), state };
}

export async function exchangeCodeForToken(code) {
  const config = getConfig();
  const response = await fetch(TOKEN_URL, {
    method: 'POST',
    headers: {
      Accept: 'application/json'
    },
    body: new URLSearchParams({
      client_id: config.githubClientId,
      client_secret: config.githubClientSecret,
      code
    })
  });
  if (!response.ok) {
    throw new Error(`GitHub token exchange failed: ${response.status}`);
  }
  const data = await response.json();
  if (!data.access_token) {
    throw new Error('GitHub did not return an access token');
  }
  return {
    accessToken: data.access_token,
    encryptedToken: encrypt(data.access_token)
  };
}

export function createOctokit(encryptedToken) {
  const token = decrypt(encryptedToken);
  return new Octokit({ auth: token });
}

export function createOctokitFromToken(token) {
  return new Octokit({ auth: token });
}

export async function listRepositories(encryptedToken) {
  const octokit = createOctokit(encryptedToken);
  const repos = await octokit.paginate(octokit.repos.listForAuthenticatedUser, {
    per_page: 100
  });
  return repos.map((repo) => ({
    id: repo.id,
    name: repo.name,
    full_name: repo.full_name,
    private: repo.private,
    default_branch: repo.default_branch
  }));
}

export async function listBranches(encryptedToken, owner, repo) {
  const octokit = createOctokit(encryptedToken);
  const branches = await octokit.paginate(octokit.repos.listBranches, {
    owner,
    repo,
    per_page: 100
  });
  return branches.map((branch) => ({
    name: branch.name,
    commit: branch.commit.sha
  }));
}

export async function getCommit(encryptedToken, owner, repo, ref) {
  const octokit = createOctokit(encryptedToken);
  const { data } = await octokit.repos.getCommit({ owner, repo, ref });
  return {
    sha: data.sha,
    message: data.commit.message,
    author: data.commit.author.name,
    date: data.commit.author.date
  };
}

export async function downloadRepository(encryptedToken, owner, repo, ref, destination) {
  const octokit = createOctokit(encryptedToken);
  await log('info', 'Downloading repository archive', { owner, repo, ref });
  const { data } = await octokit.repos.downloadTarballArchive({
    owner,
    repo,
    ref
  });
  const arrayBuffer = await data.arrayBuffer();
  await Bun.write(destination, Buffer.from(arrayBuffer));
}
