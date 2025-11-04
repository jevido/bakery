const API_BASE = import.meta.env.PUBLIC_API_URL ? import.meta.env.PUBLIC_API_URL.replace(/\/$/, '') : '';

function resolveUrl(path) {
	if (path.startsWith('http')) return path;
	return `${API_BASE}${path}`;
}

/** @param {RequestInit & { parseJson?: boolean }} options */
export async function apiFetch(path, options = {}) {
	const { parseJson = true, ...rest } = options;
	const opts = {
		credentials: 'include',
		...rest
	};

	const isJsonBody =
		opts.body &&
		typeof opts.body === 'object' &&
		!(opts.body instanceof FormData) &&
		!(opts.body instanceof URLSearchParams);

	const headers = {
		...(opts.headers || {})
	};

	if (isJsonBody) {
		headers['content-type'] = 'application/json';
	}

	opts.headers = Object.fromEntries(
		Object.entries(headers).filter(([, value]) => value != null)
	);

	if (isJsonBody) {
		opts.body = JSON.stringify(opts.body);
	}

	const response = await fetch(resolveUrl(path), opts);

	if (!response.ok) {
		let errorPayload;
		try {
			errorPayload = await response.clone().json();
		} catch {
			errorPayload = { error: response.statusText };
		}
		const error = new Error(errorPayload.error || 'Request failed');
		error.status = response.status;
		error.details = errorPayload.details;
		throw error;
	}

	if (!parseJson) {
		return response;
	}

	try {
		return await response.json();
	} catch {
		return null;
	}
}

export async function login({ email, password }) {
	return apiFetch('/api/auth/login', {
		method: 'POST',
		body: { email, password }
	});
}

export async function logout() {
	return apiFetch('/api/auth/logout', {
		method: 'POST'
	});
}

export async function fetchMe() {
	return apiFetch('/api/auth/me');
}

export async function fetchDeployments() {
	return apiFetch('/api/deployments');
}

export async function fetchSystemAnalytics() {
	return apiFetch('/api/system/analytics');
}

export async function createDeployment(payload) {
	return apiFetch('/api/deployments', {
		method: 'POST',
		body: payload
	});
}

export async function fetchGithubRepos() {
	return apiFetch('/api/github/repos');
}

export async function fetchGithubBranches(repository) {
	const [owner, repo] = repository.split('/');
	return apiFetch(`/api/github/branches?owner=${encodeURIComponent(owner)}&repo=${encodeURIComponent(repo)}`);
}

export async function redeployDeployment(id) {
	return apiFetch(`/api/deployments/${id}/deploy`, {
		method: 'POST'
	});
}

export async function restartDeployment(id) {
	return apiFetch(`/api/deployments/${id}/restart`, {
		method: 'POST'
	});
}

export async function rollbackDeployment(id, versionId) {
	return apiFetch(`/api/deployments/${id}/rollback`, {
		method: 'POST',
		body: { versionId }
	});
}

export async function updateDeploymentEnv(id, key, value) {
	return apiFetch(`/api/deployments/${id}/env`, {
		method: 'POST',
		body: { key, value }
	});
}

export async function deleteDeploymentEnv(id, key) {
	return apiFetch(`/api/deployments/${id}/env`, {
		method: 'DELETE',
		body: { key }
	});
}

export async function addDeploymentDomain(id, hostname) {
	return apiFetch(`/api/deployments/${id}/domains`, {
		method: 'POST',
		body: { hostname }
	});
}

export async function removeDeploymentDomain(id, domainId) {
	return apiFetch(`/api/deployments/${id}/domains`, {
		method: 'DELETE',
		body: { domainId }
	});
}

export async function verifyDeploymentDomain(domainId) {
	return apiFetch(`/api/domains/${domainId}/verify`, {
		method: 'POST'
	});
}
