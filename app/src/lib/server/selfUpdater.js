import { spawn } from 'bun';
import { fileURLToPath } from 'node:url';
import { TextDecoder } from 'node:util';
import { createLogger } from './logger.js';
import { getBunExecutable } from './bunPaths.js';

const logger = createLogger('self-updater');
const decoder = new TextDecoder();

const repoRoot = fileURLToPath(new URL('../../../../', import.meta.url));
const updateScriptPath = fileURLToPath(
	new URL('../../../../infrastructure/scripts/update.js', import.meta.url)
);

let activeRun = null;
let lastResult = null;

function pipeStream(stream, level, meta, streamLabel) {
	if (!stream) return;
	const reader = stream.getReader();
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
						logger[level]('Self-update output', {
							...meta,
							stream: streamLabel,
							line
						});
					}
				}
			}
			const remaining = buffer.trim();
			if (remaining) {
				logger[level]('Self-update output', {
					...meta,
					stream: streamLabel,
					line: remaining
				});
			}
		} catch (error) {
			logger.warn('Self-update stream failure', {
				...meta,
				stream: streamLabel,
				error: error.message
			});
		}
	})();
}

export function isSelfUpdateRunning() {
	return Boolean(activeRun);
}

export function getSelfUpdateStatus() {
	if (activeRun) {
		return {
			status: 'running',
			startedAt: activeRun.startedAt,
			meta: activeRun.meta
		};
	}
	return {
		status: 'idle',
		lastResult
	};
}

export function startSelfUpdate(meta = {}) {
	if (activeRun) {
		const error = new Error('Bakery update already running');
		error.code = 'UPDATE_IN_PROGRESS';
		throw error;
	}

	const bunExecutable = getBunExecutable();
	logger.info('Starting Bakery self-update', {
		...meta,
		script: updateScriptPath
	});

	let childProcess;
	try {
		childProcess = spawn([bunExecutable, updateScriptPath], {
			cwd: repoRoot,
			stdin: 'ignore',
			stdout: 'pipe',
			stderr: 'pipe',
			env: { ...process.env }
		});
	} catch (error) {
		logger.error('Failed to launch Bakery self-update', {
			...meta,
			error: error.message
		});
		throw error;
	}

	activeRun = {
		process: childProcess,
		startedAt: new Date(),
		meta
	};

	pipeStream(childProcess.stdout, 'info', meta, 'stdout');
	pipeStream(childProcess.stderr, 'warn', meta, 'stderr');

	const completion = childProcess.exited
		.then((code) => {
			const finishedAt = new Date();
			const success = code === 0;
			const result = {
				finishedAt,
				success,
				exitCode: code,
				meta
			};
			if (success) {
				logger.info('Bakery self-update completed', {
					...meta,
					exitCode: code
				});
				lastResult = result;
				activeRun = null;
				return result;
			}
			const error = new Error(`Self-update exited with code ${code}`);
			error.exitCode = code;
			throw error;
		})
		.catch((error) => {
			if (activeRun) {
				activeRun = null;
			}
			const failure = {
				finishedAt: new Date(),
				success: false,
				exitCode: typeof error.exitCode === 'number' ? error.exitCode : null,
				meta,
				error: error.message
			};
			lastResult = failure;
			logger.error('Bakery self-update failed', {
				...meta,
				error: error.message,
				exitCode: failure.exitCode
			});
			throw error;
		});

	completion.catch(() => {});
	activeRun.completion = completion;

	return {
		pid: childProcess.pid,
		startedAt: activeRun.startedAt,
		meta
	};
}

function toIsoString(value) {
	if (!value) return null;
	if (value instanceof Date) {
		return value.toISOString();
	}
	return String(value);
}

export function serializeSelfUpdateStatus(status) {
	if (!status) {
		return {
			status: 'idle',
			lastResult: null
		};
	}

	if (status.status === 'running') {
		return {
			status: 'running',
			startedAt: toIsoString(status.startedAt),
			meta: status.meta ?? null
		};
	}

	const lastResult = status.lastResult
		? {
				...status.lastResult,
				finishedAt: toIsoString(status.lastResult.finishedAt)
		  }
		: null;

	return {
		status: 'idle',
		lastResult
	};
}
