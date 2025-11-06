import { createLogger } from '../app/src/lib/server/logger.js';

const logger = createLogger('agent:api');

const API_BASE = process.env.BAKERY_AGENT_API?.replace(/\/$/, '');
const API_TOKEN = process.env.BAKERY_AGENT_TOKEN;

if (!API_BASE) {
	throw new Error('BAKERY_AGENT_API environment variable is required');
}

if (!API_TOKEN) {
	throw new Error('BAKERY_AGENT_TOKEN environment variable is required');
}

function resolveUrl(path) {
	if (path.startsWith('http')) return path;
	return `${API_BASE}${path}`;
}

async function agentFetch(path, options = {}) {
	const url = resolveUrl(path);
	const init = {
		method: 'GET',
		headers: {
			Authorization: `Bearer ${API_TOKEN}`,
			accept: 'application/json',
			...(options.headers || {})
		},
		...options
	};

	if (init.body && typeof init.body === 'object' && !(init.body instanceof ArrayBuffer)) {
		init.headers['content-type'] = 'application/json';
		init.body = JSON.stringify(init.body);
	}

	const response = await fetch(url, init);
	if (!response.ok) {
		const text = await response.text();
		await logger.error('Agent API request failed', { url, status: response.status, body: text });
		throw new Error(`Agent API request failed: ${response.status}`);
	}

	if (response.status === 204) {
		return null;
	}

	const contentType = response.headers.get('content-type') || '';
	if (contentType.includes('application/json')) {
		return response.json();
	}
	return response.text();
}

export async function heartbeat(metadata = {}) {
	return agentFetch('/api/agent/heartbeat', {
		method: 'POST',
		body: { metadata }
	});
}

export async function reserveTask() {
	const result = await agentFetch('/api/agent/tasks/reserve', { method: 'POST' });
	return result?.task ?? null;
}

export async function finishTask(id, status, error) {
	return agentFetch(`/api/agent/tasks/${id}/finish`, {
		method: 'POST',
		body: { status, error }
	});
}

export async function getDeploymentContext(deploymentId) {
	return agentFetch(`/api/agent/deployments/${deploymentId}/context`, {
		method: 'GET'
	});
}

export async function logDeployment(deploymentId, level, message, meta) {
	return agentFetch(`/api/agent/deployments/${deploymentId}/logs`, {
		method: 'POST',
		body: { level, message, meta }
	});
}

export async function updateDeploymentStatus(deploymentId, payload) {
	return agentFetch(`/api/agent/deployments/${deploymentId}/status`, {
		method: 'POST',
		body: payload
	});
}

export async function recordDeploymentVersionRemote(deploymentId, payload) {
	return agentFetch(`/api/agent/deployments/${deploymentId}/version`, {
		method: 'POST',
		body: payload
	});
}
