import { readFile } from 'node:fs/promises';
import { join } from 'node:path';
import { spawn } from 'bun';
import { getConfig } from './config.js';
import {
	recordTrafficSnapshot,
	recordDiskSnapshot,
	recordDatabaseSnapshot
} from './models/analyticsModel.js';
import { listDatabasesForDeployment } from './models/databaseModel.js';

function parseNginxLine(line) {
	const parts = line.split(' ');
	if (parts.length < 10) return null;
	const time = parts[3]?.replace('[', '');
	const bytes = Number(parts[9]) || 0;
	return { time: new Date(time.replace(':', ' ')), bytes };
}

async function getDirectorySize(path) {
	const process = spawn(['du', '-sb', path], {
		stdout: 'pipe',
		stderr: 'pipe'
	});

	const [stdout, stderr, exitCode] = await Promise.all([
		new Response(process.stdout).text(),
		new Response(process.stderr).text(),
		process.exited
	]);

	if (exitCode !== 0) {
		throw new Error(stderr || 'Failed to read directory size');
	}

	const [size] = stdout.trim().split(/\s+/);
	const value = Number(size);
	if (!Number.isFinite(value)) {
		throw new Error('Invalid directory size output');
	}
	return value;
}

export async function collectAnalytics(deployment) {
	const config = getConfig();
	const accessLog = join(config.logsDir, `${deployment.id}-${deployment.active_slot}-access.log`);
	let visits = 0;
	let bandwidth = 0;

	try {
		const content = await readFile(accessLog, 'utf8');
		const lines = content.trim().split('\n').slice(-500);
		visits = lines.length;
		bandwidth = lines.reduce((acc, line) => {
			const parsed = parseNginxLine(line);
			if (!parsed) return acc;
			return acc + parsed.bytes;
		}, 0);
	} catch {
		// ignore missing logs
	}

	await recordTrafficSnapshot({
		deploymentId: deployment.id,
		visits,
		bandwidth,
		timestamp: new Date()
	});

	try {
		const deploymentPath = join(config.buildsDir, deployment.id);
		const usedBytes = await getDirectorySize(deploymentPath);
		await recordDiskSnapshot({
			deploymentId: deployment.id,
			usedBytes,
			timestamp: new Date()
		});
	} catch {
		// ignore missing directories or du failures
	}

	const databases = await listDatabasesForDeployment(deployment.id);
	let totalSize = 0;
	databases.forEach((db) => {
		totalSize += db.size_bytes || 0;
	});

	await recordDatabaseSnapshot({
		deploymentId: deployment.id,
		databaseSize: totalSize,
		timestamp: new Date()
	});
}
