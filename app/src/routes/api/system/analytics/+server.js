import { json, error } from '@sveltejs/kit';
import { statfs } from 'node:fs/promises';
import { getConfig } from '$lib/server/config.js';
import { log } from '$lib/server/logger.js';
import { listRecentTasks } from '$lib/server/models/taskModel.js';
import { getGithubAccount } from '$lib/server/models/userModel.js';
import { getDiskUsageTrend } from '$lib/server/models/analyticsModel.js';
import { sql } from 'bun';

async function getDiskUsage(path) {
	const stats = await statfs(path);
	const total = stats.blocks * stats.blockSize;
	const free = stats.bfree * stats.blockSize;
	return { total, free, used: total - free };
}

function computeDiskForecast(history, capacity) {
	if (!capacity || history.length < 2) {
		return null;
	}

	const points = history
		.map((row) => ({
			time: new Date(row.bucket).getTime(),
			used: Number(row.used_bytes) || 0
		}))
		.filter((point) => Number.isFinite(point.time) && Number.isFinite(point.used))
		.sort((a, b) => a.time - b.time);

	if (points.length < 2) {
		return null;
	}

	const first = points[0];
	const last = points[points.length - 1];
	if (last.used <= first.used) {
		return null;
	}

	const elapsedHours = (last.time - first.time) / (1000 * 60 * 60);
	if (!Number.isFinite(elapsedHours) || elapsedHours <= 0) {
		return null;
	}

	const growthPerHour = (last.used - first.used) / elapsedHours;
	if (!Number.isFinite(growthPerHour) || growthPerHour <= 0) {
		return null;
	}

	const remaining = capacity - last.used;
	if (remaining <= 0) {
		return {
			type: 'disk',
			severity: 'critical',
			message: 'Bakery build storage is full. Free up space immediately.',
			hoursToExhaustion: 0,
			usageRatio: 1
		};
	}

	const hoursToExhaustion = remaining / growthPerHour;
	const usageRatio = last.used / capacity;
	if (!Number.isFinite(hoursToExhaustion)) {
		return null;
	}

	if (hoursToExhaustion > 168 && usageRatio < 0.85) {
		return null;
	}

	const severity = hoursToExhaustion < 24 || usageRatio > 0.95 ? 'critical' : 'warning';
	const roundedHours = Math.max(1, Math.round(hoursToExhaustion));

	return {
		type: 'disk',
		severity,
		message:
			severity === 'critical'
				? `Bakery build storage may fill within ${roundedHours} hour${roundedHours === 1 ? '' : 's'}.`
				: `Bakery build storage is trending toward full capacity in about ${roundedHours} hours.`,
		hoursToExhaustion,
		usageRatio
	};
}

export const GET = async ({ locals }) => {
	if (!locals.user) {
		throw error(401, 'Unauthorized');
	}
	const config = getConfig();
	let disk = { total: 0, free: 0, used: 0 };
	let systemDisk = { total: 0, free: 0, used: 0 };
	let deploymentStats = { total: 0, active: 0, pending: 0 };
	let databaseStats = { total: 0 };
	let domainStats = { total: 0, verified: 0 };
	let githubLinked = false;
	let predictiveAlerts = [];
	try {
		disk = await getDiskUsage(config.buildsDir);
		systemDisk = await getDiskUsage('/');
	} catch {}
	try {
		const [deploymentsRow] =
			(await sql`
        SELECT
          COUNT(*)::int AS total,
          COUNT(*) FILTER (WHERE status = 'active')::int AS active,
          COUNT(*) FILTER (WHERE status IN ('pending', 'building', 'deploying'))::int AS pending
        FROM deployments
        WHERE owner_id = ${locals.user.id}
      `) || [];
		if (deploymentsRow) deploymentStats = deploymentsRow;

		const [databasesRow] =
			(await sql`
        SELECT COUNT(*)::int AS total
        FROM deployment_databases db
        JOIN deployments d ON d.id = db.deployment_id
        WHERE d.owner_id = ${locals.user.id}
      `) || [];
		if (databasesRow) databaseStats = databasesRow;

		const [domainsRow] =
			(await sql`
        SELECT
          COUNT(*)::int AS total,
          COUNT(*) FILTER (WHERE verified)::int AS verified
        FROM deployment_domains dd
        JOIN deployments d ON d.id = dd.deployment_id
        WHERE d.owner_id = ${locals.user.id}
      `) || [];
		if (domainsRow) domainStats = domainsRow;

		const account = await getGithubAccount(locals.user.id);
		githubLinked = Boolean(account);
	} catch (err) {
		await log('warn', 'Failed to compute analytics aggregates', { error: err.message });
	}

	try {
		const diskHistory = await getDiskUsageTrend(72);
		const alert = computeDiskForecast(diskHistory, disk.total || 0);
		if (alert) {
			predictiveAlerts.push(alert);
		}
	} catch (err) {
		await log('warn', 'Failed to compute disk forecast', { error: err.message });
	}

	const tasks = await listRecentTasks(20);
	return json({
		disk,
		systemDisk,
		tasks,
		deploymentStats,
		databaseStats,
		domainStats,
		githubLinked,
		predictiveAlerts
	});
};
