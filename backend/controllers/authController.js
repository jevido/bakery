import { createSession, destroySession, verifyPassword, hashPassword, parseCookies } from '../lib/auth.js';
import { createUser, findUserByEmail, getGithubAccount, updateUserGithubLink, listUsers } from '../models/userModel.js';
import { getOAuthUrl, exchangeCodeForToken, createOctokitFromToken } from '../lib/github.js';
import { createOAuthState, consumeOAuthState } from '../models/oauthModel.js';
import { log } from '../lib/logger.js';

export const AuthController = {
  async register(ctx) {
    const body = await ctx.body;
    const existing = await listUsers();
    if (existing.length > 0 && (!ctx.user || !ctx.user.is_admin)) {
      return ctx.json({ error: 'Registration locked' }, 403);
    }
    const { email, password } = body;
    if (!email || !password) {
      return ctx.json({ error: 'Email and password required' }, 422);
    }
    const found = await findUserByEmail(email);
    if (found) {
      return ctx.json({ error: 'Email already registered' }, 409);
    }
    const passwordHash = await hashPassword(password);
    const user = await createUser({ email, passwordHash, isAdmin: existing.length === 0 });
    return ctx.json({ user: { id: user.id, email: user.email, is_admin: user.is_admin } }, 201);
  },

  async login(ctx) {
    const body = await ctx.body;
    const { email, password } = body;
    if (!email || !password) {
      return ctx.json({ error: 'Invalid credentials' }, 422);
    }
    const user = await findUserByEmail(email);
    if (!user) {
      return ctx.json({ error: 'Invalid credentials' }, 401);
    }
    const ok = await verifyPassword(password, user.password_hash);
    if (!ok) {
      return ctx.json({ error: 'Invalid credentials' }, 401);
    }
    const ip = ctx.request.headers.get('x-forwarded-for') || ctx.request.headers.get('x-real-ip');
    const ua = ctx.request.headers.get('user-agent');
    const session = await createSession(user.id, ip, ua);
    const response = await ctx.json({
      user: {
        id: user.id,
        email: user.email,
        is_admin: user.is_admin
      }
    });
    response.headers.set('Set-Cookie', session.cookie);
    return response;
  },

  async logout(ctx) {
    const sessionCookie = ctx.request.headers.get('cookie');
    if (sessionCookie) {
      const cookies = parseCookies(sessionCookie);
      if (cookies.bakery_session) {
        await destroySession(cookies.bakery_session);
      }
    }
    const response = await ctx.json({ ok: true });
    response.headers.set('Set-Cookie', 'bakery_session=; Max-Age=0; Path=/; HttpOnly; SameSite=Strict');
    return response;
  },

  async me(ctx) {
    if (!ctx.user) {
      return ctx.json({ user: null });
    }
    const github = await getGithubAccount(ctx.user.id);
    return ctx.json({
      user: {
        id: ctx.user.id,
        email: ctx.user.email,
        is_admin: ctx.user.is_admin,
        github_connected: Boolean(github)
      }
    });
  },

  async githubStart(ctx) {
    if (!ctx.user) {
      return ctx.json({ error: 'Unauthorized' }, 401);
    }
    const { url, state } = getOAuthUrl();
    await createOAuthState(ctx.user.id, 'github', state);
    return ctx.json({ url, state });
  },

  async githubCallback(ctx) {
    const { searchParams } = ctx.url;
    const code = searchParams.get('code');
    const state = searchParams.get('state');
    if (!code || !state) {
      return ctx.json({ error: 'Missing code or state' }, 422);
    }
    const record = await consumeOAuthState(state, 'github');
    if (!record) {
      return ctx.json({ error: 'Invalid OAuth state' }, 400);
    }
    try {
      const { accessToken, encryptedToken } = await exchangeCodeForToken(code);
      const octokit = createOctokitFromToken(accessToken);
      const { data } = await octokit.users.getAuthenticated();
      await updateUserGithubLink(record.user_id, data.id, encryptedToken);
      await log('info', 'GitHub account linked', { userId: record.user_id });
      return ctx.json({ ok: true });
    } catch (error) {
      await log('error', 'GitHub OAuth failed', { error: error.message });
      return ctx.json({ error: 'GitHub link failed', details: error.message }, 500);
    }
  }
};
