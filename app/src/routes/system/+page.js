export async function load({ fetch }) {
	const [analyticsRes, healthRes] = await Promise.all([
		fetch('/api/system/analytics', { credentials: 'include' }),
		fetch('/api/system/health', { credentials: 'include' })
	]);

	let analytics = {
		disk: null,
		systemDisk: null,
		tasks: [],
		deploymentStats: { total: 0, active: 0, pending: 0 },
		databaseStats: { total: 0 },
		domainStats: { total: 0, verified: 0 }
	};
	let health = null;

	if (analyticsRes.ok) {
		try {
			analytics = await analyticsRes.json();
		} catch {
			// ignore parse error
		}
	}

	if (healthRes.ok) {
		try {
			health = await healthRes.json();
		} catch {
			health = null;
		}
	}

	return { analytics, health };
}
