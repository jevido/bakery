import { spawn } from 'bun';
import { statfs } from 'node:fs/promises';
import { getConfig } from '../lib/config.js';
import { ensureConnection } from '../lib/db.js';
import { listRecentTasks } from '../models/taskModel.js';
import { log } from '../lib/logger.js';

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
    try {
      disk = await getDiskUsage(config.buildsDir);
      systemDisk = await getDiskUsage('/');
    } catch {}
    const tasks = await listRecentTasks(20);
    return ctx.json({
      disk,
      systemDisk,
      tasks
    });
  },

  async update(ctx) {
    try {
      const output = await runCommand('bash', ['scripts/update.sh'], {
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
