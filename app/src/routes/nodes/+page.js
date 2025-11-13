export async function load({ fetch }) {
	const response = await fetch('/api/nodes', { credentials: 'include' });
	let nodes = [];
	let controlPlane = null;
	if (response.ok) {
		try {
			const payload = await response.json();
			nodes = payload.nodes || [];
			controlPlane = payload.controlPlane ?? null;
		} catch {
			nodes = [];
		}
	}
	return {
		nodes,
		controlPlane
	};
}
