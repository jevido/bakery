export async function load({ fetch }) {
	const response = await fetch('/api/github/repos', { credentials: 'include' });

	let repositories = [];

	if (response.ok) {
		try {
			const payload = await response.json();
			repositories = payload.repositories || [];
		} catch {
			repositories = [];
		}
	}

	return {
		repositories
	};
}
