import { redirect } from '@sveltejs/kit';

export async function load({ fetch, url }) {
	const search = url.searchParams.toString();
	const response = await fetch(`/api/auth/github/callback?${search}`, {
		credentials: 'include'
	});

	if (!response.ok) {
		throw redirect(302, `/login?error=github`);
	}

	throw redirect(302, '/deployments/new');
}
