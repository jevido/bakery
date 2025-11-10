import { setTimeout as sleep } from 'node:timers/promises';
import { createLogger } from '../app/src/lib/server/logger.js';
import {
	reserveTask,
	finishTask,
	heartbeat,
	logDeployment,
	updateDeploymentStatus
} from './apiClient.js';
import {
	deployTask,
	activateVersionTask,
	cleanupDeploymentTask,
	stopDeploymentRuntimeTask,
	startDeploymentRuntimeTask
} from './workflows.js';

const logger = createLogger('agent');

const POLL_INTERVAL = Number(process.env.BAKERY_AGENT_POLL_INTERVAL || 3000);

const handlers = {
	async deploy(task) {
		await deployTask({
			deploymentId: task.payload.deploymentId,
			commitSha: task.payload.commitSha
		});
	},
	async restart(task) {
		const deploymentId = task.payload.deploymentId;
		await updateDeploymentStatus(deploymentId, { status: 'restarting' }).catch(() => {});
		await deployTask({ deploymentId });
	},
	async rollback(task) {
		await activateVersionTask({
			deploymentId: task.payload.deploymentId,
			version: task.payload.version
		});
	},
	async cleanup(task) {
		await cleanupDeploymentTask({ deploymentId: task.payload.deploymentId });
	},
	async stop(task) {
		await stopDeploymentRuntimeTask({ deploymentId: task.payload.deploymentId });
	},
	async start(task) {
		await startDeploymentRuntimeTask({ deploymentId: task.payload.deploymentId });
	}
};

async function processLoop() {
	await heartbeat({ state: 'boot' }).catch(() => {});
	while (true) {
		let task = null;
		try {
			task = await reserveTask();
			if (!task) {
				await heartbeat({ state: 'idle' }).catch(() => {});
				await sleep(POLL_INTERVAL);
				continue;
			}

			await heartbeat({ state: 'running', task: task.type }).catch(() => {});
			const handler = handlers[task.type];
			if (!handler) {
				throw new Error(`Unsupported task type ${task.type}`);
			}

			await handler(task);
			await finishTask(task.id, 'completed').catch(() => {});
			await heartbeat({ state: 'idle' }).catch(() => {});
		} catch (error) {
			logger.error('Task processing error', {
				error: error.message,
				task: task?.type
			});
			if (task?.payload?.deploymentId) {
				await logDeployment(task.payload.deploymentId, 'error', error.message, {
					task: task.type || 'unknown'
				}).catch(() => {});
				if (task.type === 'deploy' || task.type === 'restart') {
					await updateDeploymentStatus(task.payload.deploymentId, { status: 'failed' }).catch(
						() => {}
					);
				}
			}
			if (task) {
				await finishTask(task.id, 'failed', error.message).catch(() => {});
			}
		} finally {
			await sleep(POLL_INTERVAL);
		}
	}
}

processLoop().catch((error) => {
	logger.error('Agent loop crashed', { error: error.message });
	process.exit(1);
});
