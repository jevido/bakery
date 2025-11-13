import { mkdtemp, writeFile, chmod, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';

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
