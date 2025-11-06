export async function load({ fetch }) {
	const [reposRes, nodesRes] = await Promise.all([
		fetch('/api/github/repos', { credentials: 'include' }),
		fetch('/api/nodes', { credentials: 'include' })
	]);

	let repositories = [];
	let nodes = [];

	if (reposRes.ok) {
		try {
			const payload = await reposRes.json();
			repositories = payload.repositories || [];
		} catch {
			repositories = [];
		}
	}

	if (nodesRes.ok) {
		try {
			const payload = await nodesRes.json();
			nodes = payload.nodes || [];
		} catch {
			nodes = [];
		}
	}

	return {
		repositories,
		nodes
	};
}
