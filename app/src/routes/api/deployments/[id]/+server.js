import { json, error } from '@sveltejs/kit';
import { findDeploymentById, listVersions, listDeploymentLogs } from '$lib/server/models/deploymentModel.js';
import { listDomains } from '$lib/server/models/domainModel.js';
import { listEnvVars } from '$lib/server/models/envModel.js';
import { listDatabasesForDeployment } from '$lib/server/models/databaseModel.js';
import { listRecentTasks } from '$lib/server/models/taskModel.js';

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
  const [domains, envVars, versions, databases, logs, tasks] = await Promise.all([
    listDomains(deployment.id),
    listEnvVars(deployment.id, true),
    listVersions(deployment.id),
    listDatabasesForDeployment(deployment.id),
    listDeploymentLogs(deployment.id, 200),
    listRecentTasks(20)
  ]);
  const normalizedDeployment = {
    ...deployment,
    domains: typeof deployment.domains === 'string' ? JSON.parse(deployment.domains) : deployment.domains
  };
  return json({
    deployment: normalizedDeployment,
    domains,
    environment: envVars,
    versions,
    databases,
    logs,
    tasks: tasks.filter((task) => task.payload && task.payload.deploymentId === deployment.id)
  });
};
