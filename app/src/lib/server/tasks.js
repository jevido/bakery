import cron from 'node-cron';
import { randomUUID } from 'node:crypto';
import { reservePendingTask, finishTask } from './models/taskModel.js';
import {
	findDeploymentById,
	listDeploymentLogs,
	deleteDeployment,
	recordDeploymentLog
} from './models/deploymentModel.js';
import { getGithubAccount } from './models/userModel.js';
import {
	deploy,
	activateVersion,
	cleanupDeploymentResources,
	startDeploymentRuntime,
	stopDeploymentRuntime
} from './deployer.js';
import { collectAnalytics } from './analytics.js';
import { log } from './logger.js';
import { decrypt } from './crypto.js';
import { sql } from 'bun';

let workerRunning = false;

const handlers = {
	async deploy(task) {
	const { deploymentId, commitSha } = task.payload;
		const deployment = await findDeploymentById(deploymentId);
		if (!deployment) {
			throw new Error('Deployment not found');
		}

		const ownerAccount = await getGithubAccount(deployment.owner_id);
		const accessToken = ownerAccount ? decrypt(ownerAccount.access_token) : null;
	const result = await deploy(deployment, { commitSha, accessToken });
	await log('info', 'Deployment task finished', { deploymentId, result });
	await recordDeploymentLog(
		deploymentId,
		'info',
		`Deployment task finished ${JSON.stringify({ deploymentId, result })}`,
		{
			stream: 'system',
			deploymentId,
			result
		}
	);
	},
	async rollback(task) {
		const { deploymentId, version } = task.payload;
		const deployment = await findDeploymentById(deploymentId);
		if (!deployment) {
			throw new Error('Deployment not found');
		}
		await activateVersion(deployment, version);
	},
	async analytics(task) {
		const { deploymentId } = task.payload;
		const deployment = await findDeploymentById(deploymentId);
		if (!deployment) {
			return;
		}
		await collectAnalytics(deployment);
	},
	async restart(task) {
		const { deploymentId } = task.payload;
		await sql`
      UPDATE deployments
      SET status = 'restarting', updated_at = NOW()
      WHERE id = ${deploymentId}
    `;
		await handlers.deploy({ payload: { deploymentId } });
	},
	async cleanup(task) {
		const { deploymentId } = task.payload;
		const deployment = await findDeploymentById(deploymentId);
		if (!deployment) {
			return;
		}
		await cleanupDeploymentResources(deployment);
		await deleteDeployment(deploymentId);
	},
	async stop(task) {
		const { deploymentId } = task.payload;
		const deployment = await findDeploymentById(deploymentId);
		if (!deployment) {
			throw new Error('Deployment not found');
		}
		await stopDeploymentRuntime(deployment);
	},
	async start(task) {
		const { deploymentId } = task.payload;
		const deployment = await findDeploymentById(deploymentId);
		if (!deployment) {
			throw new Error('Deployment not found');
		}
		await startDeploymentRuntime(deployment);
	}
};

async function processTask() {
	const task = await reservePendingTask(null, 'control-plane');
	if (!task) {
		return;
	}
	const deploymentId = task.payload?.deploymentId;
	try {
		const handler = handlers[task.type];
		if (!handler) {
			throw new Error(`Unknown task type ${task.type}`);
		}
		if (deploymentId) {
			await recordDeploymentLog(deploymentId, 'info', `Task ${task.type} started`, {
				stream: 'system',
				taskId: task.id,
				payload: task.payload
			});
		}
		await handler(task);
		await finishTask(task.id, 'completed');
		if (deploymentId) {
			await recordDeploymentLog(deploymentId, 'info', `Task ${task.type} completed`, {
				stream: 'system',
				taskId: task.id
			});
		}
	} catch (error) {
		await log('error', 'Task failed', { taskId: task.id, error: error.message });
		await finishTask(task.id, 'failed', error.message);
		if (deploymentId) {
			await recordDeploymentLog(deploymentId, 'error', `Task ${task.type} failed`, {
				stream: 'system',
				taskId: task.id,
				error: error.message
			});
		}
	}
}

export function startTaskWorker() {
	if (workerRunning) return;
	workerRunning = true;
	setInterval(() => {
		processTask().catch((error) =>
			log('error', 'Task processing loop failed', { error: error.message })
		);
	}, 1000);
}

export function scheduleAnalyticsCollector() {
	cron.schedule('*/5 * * * *', async () => {
		try {
			const deployments = await sql`
        SELECT *
        FROM deployments
        WHERE status = 'running' AND node_id IS NULL
      `;
			for (const deployment of deployments) {
				await collectAnalytics(deployment);
			}
		} catch (error) {
			await log('error', 'Analytics collection failed', { error: error.message });
		}
	});
}

export function scheduleCrashDetector() {
	cron.schedule('*/2 * * * *', async () => {
		const deployments = await sql`SELECT * FROM deployments`;
		for (const deployment of deployments) {
			if (deployment.status === 'failed') continue;
			const logs = await listDeploymentLogs(deployment.id, 5);
			const crashed = logs.some(
				(log) => log.level === 'error' && /crash|unhandled/i.test(log.message)
			);
			if (crashed) {
				await log('warn', 'Detected crash, scheduling restart', {
					deploymentId: deployment.id
				});
				await sql`
          INSERT INTO tasks (id, type, payload, status, node_id)
          VALUES (${randomUUID()}, 'restart', ${JSON.stringify({ deploymentId: deployment.id })}::jsonb, 'pending', ${deployment.node_id})
        `;
			}
		}
	});
}
