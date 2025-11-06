import { spawn } from 'bun';
import { getConfig } from './config.js';
import { log } from './logger.js';

async function runSystemctl(args) {
	const process = spawn(['systemctl', ...args], {
		stdin: 'ignore',
		stdout: 'pipe',
		stderr: 'pipe'
	});
	const output = await new Response(process.stdout).text();
	const errorOutput = await new Response(process.stderr).text();
	if (process.exitCode !== 0) {
		await log('error', 'systemctl failed', { args, error: errorOutput.trim() });
		throw new Error(`systemctl failed: ${errorOutput}`);
	}
	return output.trim();
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
