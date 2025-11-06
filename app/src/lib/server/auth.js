import { nanoid } from 'nanoid';
import { getConfig } from './config.js';
import { sql } from 'bun';
import { log } from './logger.js';

const SESSION_COOKIE = 'bakery_session';

export async function hashPassword(password) {
	return Bun.password.hash(password, {
		algorithm: 'bcrypt',
		cost: 12
	});
}

export async function verifyPassword(password, hash) {
	return Bun.password.verify(password, hash);
}

export function parseCookies(header = '') {
	return header.split(';').reduce((acc, part) => {
		const [key, value] = part.trim().split('=');
		if (key) {
			acc[key] = decodeURIComponent(value || '');
		}
		return acc;
	}, {});
}

export async function createSession(userId, ipAddress = null, userAgent = null) {
	const token = nanoid(48);
	const expiresAt = new Date(Date.now() + 1000 * 60 * 60 * 24 * 7);
	await sql`
    INSERT INTO sessions (id, user_id, token, expires_at, last_ip, user_agent)
    VALUES (${nanoid()}, ${userId}, ${token}, ${expiresAt}, ${ipAddress}, ${userAgent})
  `;

	const config = getConfig();
	const secure = config.environment === 'production';
	const maxAge = 60 * 60 * 24 * 7;

	return {
		token,
		expiresAt,
		cookie: `${SESSION_COOKIE}=${encodeURIComponent(token)}`,
		cookieOptions: {
			path: '/',
			httpOnly: true,
			sameSite: 'strict',
			secure,
			maxAge
		}
	};
}

export async function getSession(token) {
	const rows = await sql`
    SELECT s.*, u.email, u.id as user_id, u.is_admin
    FROM sessions s
    JOIN users u ON u.id = s.user_id
    WHERE s.token = ${token} AND s.expires_at > NOW()
  `;

	const session = rows[0];

	return session;
}

export async function destroySession(token) {
	await sql`DELETE FROM sessions WHERE token = ${token}`;
}

export async function authenticateRequest(request) {
	const cookieHeader = request.headers.get('cookie') || '';
	const cookies = parseCookies(cookieHeader);
	const token = cookies[SESSION_COOKIE];
	if (!token) {
		return null;
	}
	try {
		const session = await getSession(token);
		if (!session) {
			return null;
		}
		return {
			id: session.user_id,
			email: session.email,
			is_admin: session.is_admin,
			sessionToken: token
		};
	} catch (error) {
		await log('error', 'Failed to authenticate request', { error: error.message });
		return null;
	}
}

export function requireAuth(handler) {
	return async (ctx) => {
		if (!ctx.user) {
			return ctx.json({ error: 'Unauthorized' }, 401);
		}
		return handler(ctx);
	};
}
