import { appendFile, mkdir } from 'node:fs/promises';
import { join } from 'node:path';
import { getConfig } from './config.js';

const logLevels = ['debug', 'info', 'warn', 'error'];

async function ensureDir(path) {
	await mkdir(path, { recursive: true });
}

export async function log(level, message, meta = {}) {
	if (!logLevels.includes(level)) {
		throw new Error(`Invalid log level ${level}`);
	}

	const config = getConfig();
	await ensureDir(config.logsDir);
	const timestamp = new Date().toISOString();
	const payload = JSON.stringify(meta);
	const line = `[${timestamp}] [${level.toUpperCase()}] ${message} ${payload}\n`;
	await appendFile(join(config.logsDir, 'bakery.log'), line);

	if (level === 'error' || config.environment !== 'production') {
		console[level === 'error' ? 'error' : 'log'](line.trim());
	}

	const deploymentId = meta?.deploymentId;
	if (deploymentId) {
		try {
			const { recordDeploymentLog } = await import('./models/deploymentModel.js');
			const metadata = {
				...meta,
				stream: meta.stream || 'system'
			};
			await recordDeploymentLog(deploymentId, level, message, metadata);
		} catch (error) {
			console.error('Failed to mirror log to deployment history', error);
		}
	}
}

export function createLogger(namespace) {
	return {
		debug: (message, meta) => log('debug', `[${namespace}] ${message}`, meta),
		info: (message, meta) => log('info', `[${namespace}] ${message}`, meta),
		warn: (message, meta) => log('warn', `[${namespace}] ${message}`, meta),
		error: (message, meta) => log('error', `[${namespace}] ${message}`, meta)
	};
}
