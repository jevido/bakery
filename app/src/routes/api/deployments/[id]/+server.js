import { json, error } from '@sveltejs/kit';
import {
	findDeploymentById,
	listVersions,
	listDeploymentLogs,
	deleteDeployment
} from '$lib/server/models/deploymentModel.js';
import { listDomains } from '$lib/server/models/domainModel.js';
import { listEnvVars } from '$lib/server/models/envModel.js';
import { listDatabasesForDeployment } from '$lib/server/models/databaseModel.js';
import { listRecentTasks } from '$lib/server/models/taskModel.js';
import { getResourceSummary } from '$lib/server/models/analyticsModel.js';
import { getLocalResolutionHint } from '$lib/server/domainUtils.js';
import { getRuntimeStatus } from '$lib/server/runtimeStatus.js';

export const GET = async ({ params, locals }) => {
	if (!locals.user) {
		throw error(401, 'Unauthorized');
	}
	const deployment = await findDeploymentById(params.id);
	if (!deployment) {
		throw error(404, 'Not found');
	}
	if (deployment.owner_id && deployment.owner_id !== locals.user.id) {
		throw error(403, 'Forbidden');
	}
	const [domains, envVars, versions, databases, logs, tasks, resourceSummary, runtimeStatus] = await Promise.all([
		listDomains(deployment.id),
		listEnvVars(deployment.id, true),
		listVersions(deployment.id),
		listDatabasesForDeployment(deployment.id),
		listDeploymentLogs(deployment.id, 200),
		listRecentTasks(20),
		getResourceSummary(deployment.id),
		getRuntimeStatus(deployment)
	]);
	const { node_name, node_status, ...deploymentRest } = deployment;
	const normalizedDeployment = {
		...deploymentRest,
		domains:
			typeof deployment.domains === 'string' ? JSON.parse(deployment.domains) : deployment.domains,
		node:
			deploymentRest.node_id && node_name
				? {
						id: deploymentRest.node_id,
						name: node_name,
						status: node_status
					}
				: null
	};
	const normalizedDomains = domains.map((domain) => ({
		...domain,
		resolution_hint: getLocalResolutionHint(domain.hostname)
	}));

	return json({
		deployment: normalizedDeployment,
		domains: normalizedDomains,
		environment: envVars,
		versions,
		databases,
		logs,
		tasks: tasks.filter((task) => task.payload && task.payload.deploymentId === deployment.id),
		resourceSummary,
		runtimeStatus
	});
};

export const DELETE = async ({ params, locals }) => {
	if (!locals.user) {
		throw error(401, 'Unauthorized');
	}

	const deployment = await findDeploymentById(params.id);
	if (!deployment) {
		throw error(404, 'Not found');
	}

	if (deployment.owner_id && deployment.owner_id !== locals.user.id) {
		throw error(403, 'Forbidden');
	}

	await deleteDeployment(deployment.id);

	return json({ success: true });
};
