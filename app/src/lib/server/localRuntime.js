import { spawn } from 'bun';
import { getConfig } from './config.js';
import { createLogger } from './logger.js';

const logger = createLogger('local-runtime');
const processes = new Map();

export function isLocalModeEnabled() {
	return Boolean(getConfig().localMode);
}

function pipeStream(stream, level, serviceName) {
	if (!stream) return;
	const reader = stream.getReader();
	const decoder = new TextDecoder();
	let buffer = '';

	(async () => {
		try {
			while (true) {
				const { value, done } = await reader.read();
				if (done) break;
				buffer += decoder.decode(value, { stream: true });
				let newlineIndex;
				while ((newlineIndex = buffer.indexOf('\n')) !== -1) {
					const line = buffer.slice(0, newlineIndex).trim();
					buffer = buffer.slice(newlineIndex + 1);
					if (line) {
						logger[level](`(${serviceName}) ${line}`, { service: serviceName });
					}
				}
			}
			const trailing = buffer.trim();
			if (trailing) {
				logger[level](`(${serviceName}) ${trailing}`, { service: serviceName });
			}
		} catch (error) {
			logger.error('Local runtime stream failure', {
				service: serviceName,
				error: error.message
			});
		}
	})();
}

export async function startLocalService(
	serviceName,
	{ cwd, env, command = 'bun', args = ['run', 'start'] }
) {
	if (!isLocalModeEnabled()) {
		throw new Error('startLocalService called outside local mode');
	}
	await stopLocalService(serviceName);

	const subprocess = spawn([command, ...args], {
		cwd,
		stdin: 'ignore',
		stdout: 'pipe',
		stderr: 'pipe',
		env: {
			...process.env,
			...env
		}
	});
	processes.set(serviceName, subprocess);
	pipeStream(subprocess.stdout, 'info', serviceName);
	pipeStream(subprocess.stderr, 'warn', serviceName);
	subprocess.exited
		.then((code) => {
			if (processes.get(serviceName) === subprocess) {
				processes.delete(serviceName);
			}
			logger.info('Local service exited', { service: serviceName, code });
		})
		.catch((error) => {
			logger.warn('Local service exit wait failed', {
				service: serviceName,
				error: error.message
			});
		});
	return subprocess;
}

export async function stopLocalService(serviceName) {
	const existing = processes.get(serviceName);
	if (!existing) return;
	try {
		existing.kill();
	} catch (error) {
		logger.warn('Failed to stop local service', { service: serviceName, error: error.message });
	}
	processes.delete(serviceName);
}

export function getLocalServiceStatus(serviceName) {
	return processes.has(serviceName) ? 'active' : 'inactive';
}
