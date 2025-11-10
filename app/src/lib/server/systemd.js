import { spawn } from 'bun';
import { getConfig } from './config.js';
import { log } from './logger.js';
import { getLocalServiceStatus } from './localRuntime.js';

function stripAnsi(value = '') {
	return value.replace(
		/[\u001b\u009b][[()#;?]*(?:[0-9]{1,4}(?:;[0-9]{0,4})*)?[0-9A-PR-TZcf-ntqry=><~]/g,
		''
	);
}

async function runSystemctl(args) {
	const config = getConfig();
	if (config.localMode) {
		await log('info', 'Local mode: skipping systemctl invocation', { args });
		return 'skipped';
	}
	const child = spawn(['systemctl', ...args], {
		stdin: 'ignore',
		stdout: 'pipe',
		stderr: 'pipe'
	});
	const [stdout, stderr] = await Promise.all([
		child.stdout ? new Response(child.stdout).text() : '',
		child.stderr ? new Response(child.stderr).text() : ''
	]);
	const exitCode = await child.exited;
	if (exitCode !== 0) {
		const cleanErr = stripAnsi(stderr).trim();
		await log('error', 'systemctl failed', { args, error: cleanErr });
		throw new Error(`systemctl failed: ${cleanErr || 'unknown error'}`);
	}
	return stdout.trim();
}

export async function restartService(service) {
	return runSystemctl(['restart', service]);
}

export async function startService(service) {
	return runSystemctl(['start', service]);
}

export async function stopService(service) {
	return runSystemctl(['stop', service]);
}

export async function reloadDaemon() {
	return runSystemctl(['daemon-reload']);
}

export async function enableService(service) {
	return runSystemctl(['enable', service]);
}

export async function serviceStatus(service) {
	const config = getConfig();
	if (config.localMode) {
		return getLocalServiceStatus(service.replace(/\.service$/, ''));
	}
	try {
		const output = await runSystemctl(['is-active', service]);
		return output === 'active' ? 'active' : 'inactive';
	} catch (error) {
		return 'unknown';
	}
}

export function serviceNameForDeployment(deploymentId, slot) {
	const config = getConfig();
	return `bakery-deployment-${deploymentId}-${slot}`;
}
