import { access } from 'node:fs/promises';
import { join } from 'node:path';
import { spawn } from 'bun';
import { log } from './logger.js';
import { spawn } from 'bun';

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
	const child = spawn('docker', args, {
		stdout: 'pipe',
		stderr: 'pipe'
	});
	let stdout = '';
	let stderr = '';
	child.stdout.on('data', (chunk) => {
		const text = chunk.toString();
		stdout += text;
		log('info', 'docker stdout', { chunk: text }).catch(() => {});
	});
	child.stderr.on('data', (chunk) => {
		const text = chunk.toString();
		stderr += text;
		log('info', 'docker stderr', { chunk: text }).catch(() => {});
	});
	const exitCode = await new Promise((resolve) => {
		child.on('close', resolve);
	});
	if (exitCode !== 0) {
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
