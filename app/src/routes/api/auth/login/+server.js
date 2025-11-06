import { json, error } from '@sveltejs/kit';
import { findUserByEmail } from '$lib/server/models/userModel.js';
import { verifyPassword, createSession } from '$lib/server/auth.js';

export const POST = async ({ request, cookies, getClientAddress, locals }) => {
	const body = await request.json().catch(() => null);
	const email = body?.email?.trim();
	const password = body?.password ?? '';

	if (!email || !password) {
		throw error(422, 'Invalid credentials');
	}

	const user = await findUserByEmail(email);
	if (!user) {
		throw error(401, 'Invalid credentials');
	}

	const ok = await verifyPassword(password, user.password_hash);
	if (!ok) {
		throw error(401, 'Invalid credentials');
	}

	const clientAddress = getClientAddress();
	const userAgent = request.headers.get('user-agent');
	const session = await createSession(user.id, clientAddress, userAgent);

	cookies.set('bakery_session', session.token, session.cookieOptions);
	locals.user = {
		id: user.id,
		email: user.email,
		is_admin: user.is_admin
	};

	return json({
		user: {
			id: user.id,
			email: user.email,
			is_admin: user.is_admin
		}
	});
};
