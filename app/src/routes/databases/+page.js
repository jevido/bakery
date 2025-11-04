export async function load({ fetch }) {
	const response = await fetch('/api/databases', {
		credentials: 'include'
	});

	let databases = [];

	if (response.ok) {
		try {
			const payload = await response.json();
			databases = payload.databases ?? [];
		} catch {
			databases = [];
		}
	}

	return { databases };
}
