import { redirect } from '@sveltejs/kit';

export async function load({ fetch, url, data }) {
	let user = data?.user ?? null;

	if (!user) {
		try {
			const res = await fetch('/api/auth/me', { credentials: 'include' });
			if (res.ok) {
				const payload = await res.json();
				user = payload.user;
			}
		} catch {
			user = null;
		}
	}

	const pathname = url.pathname;
	const isLoginRoute = pathname === '/login';

	if (!user && !isLoginRoute) {
		throw redirect(302, '/login');
	}

	if (user && isLoginRoute) {
		throw redirect(302, '/');
	}

	return { user };
}
