import { json, error } from '@sveltejs/kit';
import { statfs } from 'node:fs/promises';
import { getConfig } from '$lib/server/config.js';
import { log } from '$lib/server/logger.js';
import { listRecentTasks } from '$lib/server/models/taskModel.js';
import { getGithubAccount } from '$lib/server/models/userModel.js';
import { sql } from 'bun';

async function getDiskUsage(path) {
  const stats = await statfs(path);
  const total = stats.blocks * stats.blockSize;
  const free = stats.bfree * stats.blockSize;
  return { total, free, used: total - free };
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

  const tasks = await listRecentTasks(20);
  return json({
    disk,
    systemDisk,
    tasks,
    deploymentStats,
    databaseStats,
    domainStats,
    githubLinked
  });
};
