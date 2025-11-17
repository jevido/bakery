import { access } from 'node:fs/promises';
import { join } from 'node:path';
import { spawn } from 'bun';
import { log } from './logger.js';

export async function detectDockerfile(repoPath, dockerfilePath = 'Dockerfile') {
	const target = join(repoPath, dockerfilePath);
	try {
		await access(target);
		return true;
	} catch {
		return false;
	}
}

async function runDockerCommand(args) {
	await log('info', 'Executing docker command', { args });
	const process = spawn(['docker', ...args], {
		stdout: 'pipe',
		stderr: 'pipe'
	});
	const stdout = await new Response(process.stdout).text();
	const stderr = await new Response(process.stderr).text();
	if (process.exitCode !== 0) {
		await log('error', 'Docker command failed', { args, stderr });
		throw new Error(stderr);
	}
	return stdout.trim();
}

export async function buildImage({ context, tag, dockerfile }) {
	const args = ['build', '-t', tag];
	if (dockerfile) {
		args.push('-f', dockerfile);
	}
	args.push(context);
	return runDockerCommand(args);
}

export async function runContainer({ image, name, env = {}, portMapping, volumes = [] }) {
	const args = ['run', '-d', '--name', name, '--restart', 'always'];
	Object.entries(env).forEach(([key, value]) => {
		args.push('-e', `${key}=${value}`);
	});
	if (portMapping) {
		args.push('-p', `${portMapping.host}:${portMapping.container}`);
	}
	volumes.forEach((volume) => {
		args.push('-v', volume);
	});
	args.push(image);
	return runDockerCommand(args);
}

export async function stopAndRemoveContainer(name) {
	try {
		await runDockerCommand(['rm', '-f', name]);
	} catch (error) {
		if (!error.message.includes('No such container')) {
			throw error;
		}
	}
}
