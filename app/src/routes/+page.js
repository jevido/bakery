export async function load({ fetch }) {
	const [analyticsRes, deploymentsRes] = await Promise.all([
		fetch('/api/system/analytics', { credentials: 'include' }),
		fetch('/api/deployments', { credentials: 'include' })
	]);

	let analytics = { disk: null, systemDisk: null, tasks: [] };
	let deployments = [];

	if (analyticsRes.ok) {
		try {
			const payload = await analyticsRes.json();
			analytics = payload;
		} catch {
			// ignore parse error
		}
	}

	if (deploymentsRes.ok) {
		try {
			const payload = await deploymentsRes.json();
			deployments = payload.deployments || [];
		} catch {
			// ignore parse error
		}
	}

	return {
		analytics,
		deployments
	};
}
