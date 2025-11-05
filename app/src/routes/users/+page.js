import { redirect, error } from '@sveltejs/kit';

export async function load({ fetch }) {
	const response = await fetch('/api/users', { credentials: 'include' });

	if (response.status === 401) {
		throw redirect(302, '/login');
	}

	if (response.status === 403) {
		throw error(403, 'You are not allowed to manage users.');
	}

	if (!response.ok) {
		throw error(response.status, 'Failed to load users');
	}

	const payload = await response.json().catch(() => ({ users: [] }));
	return { users: payload.users ?? [] };
}
