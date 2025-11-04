export async function load({ fetch }) {
	const response = await fetch('/api/deployments', { credentials: 'include' });

	let deployments = [];

	if (response.ok) {
		try {
			const payload = await response.json();
			deployments = payload.deployments || [];
		} catch {
			deployments = [];
		}
	}

	return {
		deployments
	};
}
