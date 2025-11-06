export async function load({ fetch }) {
	const response = await fetch('/api/nodes', { credentials: 'include' });
	let nodes = [];
	let installBlocked = false;
	let installWarning = null;
	let apiBase = null;
	let controlPlane = null;
	if (response.ok) {
		try {
			const payload = await response.json();
			nodes = payload.nodes || [];
			installBlocked = Boolean(payload.installBlocked);
			installWarning = payload.installWarning ?? null;
			apiBase = payload.apiBase ?? null;
			controlPlane = payload.controlPlane ?? null;
		} catch {
			nodes = [];
		}
	}
	return {
		nodes,
		controlPlane,
		install: {
			blocked: installBlocked,
			warning: installWarning,
			apiBase
		}
	};
}
