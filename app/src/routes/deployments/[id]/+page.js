import { error } from '@sveltejs/kit';

export async function load({ fetch, params }) {
	const response = await fetch(`/api/deployments/${params.id}`, {
		credentials: 'include'
	});

	if (response.status === 404) {
		throw error(404, 'Deployment not found');
	}

	if (!response.ok) {
		throw error(response.status || 500, 'Failed to load deployment');
	}

	const payload = await response.json();

	return {
		id: params.id,
		...payload
	};
}
