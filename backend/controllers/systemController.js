import { spawn } from 'bun';
import { statfs } from 'node:fs/promises';
import { getConfig } from '../lib/config.js';
import { ensureConnection, sql } from '../lib/db.js';
import { listRecentTasks } from '../models/taskModel.js';
import { log } from '../lib/logger.js';
import { getGithubAccount } from '../models/userModel.js';

async function runCommand(command, args, options = {}) {
  const process = spawn([command, ...args], {
    stdout: 'pipe',
    stderr: 'pipe',
    cwd: options.cwd
  });
  const stdout = await new Response(process.stdout).text();
  const stderr = await new Response(process.stderr).text();
  if (process.exitCode !== 0) {
    throw new Error(stderr);
  }
  return stdout.trim();
}

async function getDiskUsage(path) {
  const stats = await statfs(path);
  const total = stats.blocks * stats.blockSize;
  const free = stats.bfree * stats.blockSize;
  return { total, free, used: total - free };
}

export const SystemController = {
  async health(ctx) {
    await ensureConnection();
    return ctx.json({ status: 'ok', time: new Date().toISOString() });
  },

  async analytics(ctx) {
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
          WHERE owner_id = ${ctx.user.id}
        `) || [];
      if (deploymentsRow) {
        deploymentStats = deploymentsRow;
      }

      const [databasesRow] =
        (await sql`
          SELECT COUNT(*)::int AS total
          FROM deployment_databases db
          JOIN deployments d ON d.id = db.deployment_id
          WHERE d.owner_id = ${ctx.user.id}
        `) || [];
      if (databasesRow) {
        databaseStats = databasesRow;
      }

      const [domainsRow] =
        (await sql`
          SELECT
            COUNT(*)::int AS total,
            COUNT(*) FILTER (WHERE verified)::int AS verified
          FROM deployment_domains dd
          JOIN deployments d ON d.id = dd.deployment_id
          WHERE d.owner_id = ${ctx.user.id}
        `) || [];
      if (domainsRow) {
        domainStats = domainsRow;
      }

      const account = await getGithubAccount(ctx.user.id);
      githubLinked = Boolean(account);
    } catch (error) {
      await log('warn', 'Failed to compute analytics aggregates', { error: error.message });
    }
    const tasks = await listRecentTasks(20);
    return ctx.json({
      disk,
      systemDisk,
      tasks,
      deploymentStats,
      databaseStats,
      domainStats,
      githubLinked
    });
  },

  async update(ctx) {
    try {
      const output = await runCommand('bash', ['infrastructure/scripts/update.sh'], {
        cwd: process.cwd()
      });
      await log('info', 'Update executed by user', { userId: ctx.user.id });
      return ctx.json({ ok: true, output });
    } catch (error) {
      await log('error', 'Update failed', { error: error.message });
      return ctx.json({ error: 'Update failed', details: error.message }, 500);
    }
  }
};
