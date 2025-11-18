import { mkdtemp, writeFile, chmod, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { TextDecoder } from 'node:util';

const decoder = new TextDecoder();

async function writeTempKey(privateKey) {
	const dir = await mkdtemp(join(tmpdir(), 'bakery-ssh-'));
	const keyPath = join(dir, 'id');
	await writeFile(keyPath, privateKey, { mode: 0o600 });
	await chmod(keyPath, 0o600);
	return { dir, keyPath };
}

export async function runSshCommand({ host, port = 22, user = 'bakery-agent', privateKey }, command) {
	if (!host || !privateKey) {
		throw new Error('Missing SSH host or key');
	}

	const { dir, keyPath } = await writeTempKey(privateKey);
	const args = [
		'-i',
		keyPath,
		'-p',
		String(port),
		'-o',
		'BatchMode=yes',
		'-o',
		'StrictHostKeyChecking=no',
		`${user}@${host}`,
		'bash',
		'-lc',
		command
	];

	const child = Bun.spawn(['ssh', ...args], {
		stdout: 'pipe',
		stderr: 'pipe'
	});

	let stdout = '';
	let stderr = '';
	if (child.stdout) {
		stdout = await new Response(child.stdout).text();
	}
	if (child.stderr) {
		stderr = await new Response(child.stderr).text();
	}

	const exitCode = await child.exited;
	await rm(dir, { recursive: true, force: true });

	if (exitCode !== 0) {
		const error = new Error('SSH execution failed');
		error.exitCode = exitCode;
		error.stdout = stdout.trim();
		error.stderr = stderr.trim();
		throw error;
	}

	return {
		stdout: stdout.trim(),
		stderr: stderr.trim()
	};
}

export function shellEscape(value) {
	if (value === null || value === undefined) return "''";
	const stringValue = String(value);
	if (stringValue === '') return "''";
	return `'${stringValue.replace(/'/g, `'"'"'`)}'`;
}

function buildRemoteCommand(command, { cwd, env }) {
	let final = command;
	if (env && Object.keys(env).length) {
		const exports = Object.entries(env)
			.map(([key, value]) => `${key}=${shellEscape(value)}`)
			.join(' ');
		final = `${exports} ${final}`;
	}
	if (cwd) {
		final = `cd ${shellEscape(cwd)} && ${final}`;
	}
	return final;
}

async function streamOutput(stream, label, hook) {
	if (!stream) return '';
	const reader = stream.getReader();
	let buffer = '';
	let output = '';
	while (true) {
		const { value, done } = await reader.read();
		if (done) break;
		const chunk = decoder.decode(value);
		output += chunk;
		buffer += chunk;
		const lines = buffer.split(/\r?\n/);
		buffer = lines.pop() ?? '';
		for (const line of lines) {
			const normalized = line.replace(/\r/g, '');
			if (!normalized) continue;
			await hook?.(label, normalized);
		}
	}
	const remaining = buffer.replace(/\r/g, '').trim();
	if (remaining) {
		await hook?.(label, remaining);
	}
	return output;
}

function isShellNoise(line) {
	if (!line) return true;
	const trimmed = line.trim();
	if (!trimmed) return true;
	// Lines like BASH=/usr/bin/bash or PIPESTATUS=([0]="1")
	if (/^[A-Z0-9_]+(\[[^\]]+\])?\s*=/.test(trimmed)) {
		return true;
	}
	return false;
}

async function runStreamingSsh({ sshArgs, command, hooks, timeoutMs, acceptExitCodes = [0], strict }) {
	const prefixed = strict === false ? command : `set -euo pipefail; ${command}`;
	const child = Bun.spawn(['ssh', ...sshArgs, 'bash', '-lc', prefixed], {
		stdout: 'pipe',
		stderr: 'pipe'
	});

	let timeoutId;
	let timedOut = false;
	if (timeoutMs && timeoutMs > 0) {
		timeoutId = setTimeout(() => {
			timedOut = true;
			child.kill?.();
		}, timeoutMs);
		if (typeof timeoutId?.unref === 'function') {
			timeoutId.unref();
		}
	}

	const [stdout, stderr] = await Promise.all([
		streamOutput(child.stdout, 'stdout', hooks?.onLine),
		streamOutput(child.stderr, 'stderr', hooks?.onLine)
	]);
	const exitCode = await child.exited;
	if (timeoutId) clearTimeout(timeoutId);

	if (timedOut) {
		const error = new Error(`SSH command timed out after ${timeoutMs}ms`);
		error.stdout = stdout.trim();
		error.stderr = stderr.trim();
		throw error;
	}

	if (!acceptExitCodes.includes(exitCode)) {
		const error = new Error(`SSH command failed with exit code ${exitCode}`);
		error.stdout = stdout.trim();
		error.stderr = stderr.trim();
		error.exitCode = exitCode;
		throw error;
	}

	return {
		exitCode,
		stdout: stdout.trim(),
		stderr: stderr.trim()
	};
}

export async function createSshRunner(config, hooks = {}) {
	if (!config?.host || !config?.privateKey) {
		throw new Error('Missing SSH host or key');
	}
	const { dir, keyPath } = await writeTempKey(config.privateKey);
	const user = config.user || 'bakery-agent';
	const port = config.port || 22;
	const sshArgs = [
		'-i',
		keyPath,
		'-p',
		String(port),
		'-o',
		'BatchMode=yes',
		'-o',
		'StrictHostKeyChecking=no',
		`${user}@${config.host}`
	];

	async function exec(command, options = {}) {
		const finalCommand = buildRemoteCommand(command, {
			cwd: options.cwd,
			env: options.env
		});
		if (options.log !== false) {
			await hooks.onCommandStart?.(finalCommand);
		}
		const result = await runStreamingSsh({
			sshArgs,
			command: finalCommand,
			hooks: {
				onLine: async (label, line) => {
					if (isShellNoise(line)) return;
					if (label === 'stdout') {
						await hooks.onStdout?.(line);
					} else {
						await hooks.onStderr?.(line);
					}
				}
			},
			timeoutMs: options.timeoutMs,
			acceptExitCodes: options.acceptExitCodes || [0],
			strict: options.strict ?? true
		});
		return result;
	}

	async function readFile(path, options = {}) {
		const result = await exec(`cat ${shellEscape(path)}`, {
			...options,
			log: options.log ?? false
		});
		return result.stdout;
	}

	async function writeFile(path, contents, options = {}) {
		const encoded = Buffer.from(contents, 'utf8').toString('base64');
		const dirPath = path.split('/').slice(0, -1).join('/') || '.';
		const mkdirCmd = `${options.sudo ? 'sudo ' : ''}mkdir -p ${shellEscape(dirPath)}`;
		await exec(mkdirCmd, { log: options.log ?? false });
		const decodeCmd = `${options.sudo ? 'sudo ' : ''}bash -c "printf %s ${shellEscape(
			encoded
		)} | base64 -d > ${shellEscape(path)}"`;
		await exec(decodeCmd, { log: options.log ?? false });
		if (options.mode) {
			await exec(`${options.sudo ? 'sudo ' : ''}chmod ${options.mode.toString(8)} ${shellEscape(path)}`, {
				log: options.log ?? false
			});
		}
	}

	async function fileExists(path, options = {}) {
		const result = await exec(`[ -f ${shellEscape(path)} ]`, {
			...options,
			acceptExitCodes: [0, 1],
			log: options.log ?? false
		});
		return result.exitCode === 0;
	}

	async function dispose() {
		await rm(dir, { recursive: true, force: true }).catch(() => {});
	}

	return {
		exec,
		readFile,
		writeFile,
		fileExists,
		dispose
	};
}
