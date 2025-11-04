import { z } from 'zod';
import {
  listDeploymentsForUser,
  createDeployment,
  findDeploymentById,
  listVersions,
  recordDeploymentLog,
  listDeploymentLogs
} from '../models/deploymentModel.js';
import { addDomain, listDomains, deleteDomain } from '../models/domainModel.js';
import { upsertEnvVar, removeEnvVar, listEnvVars } from '../models/envModel.js';
import { createTask, listRecentTasks } from '../models/taskModel.js';
import {
  createDatabaseRecord,
  listDatabasesForDeployment,
  deleteDatabase
} from '../models/databaseModel.js';
import { provisionDatabase, dropDatabase } from '../lib/postgresAdmin.js';

const encoder = new TextEncoder();

const createDeploymentSchema = z.object({
  name: z.string().min(3),
  repository: z.string(),
  branch: z.string(),
  domains: z.array(z.string()).default([]),
  environment: z.record(z.string()).default({}),
  enableBlueGreen: z.boolean().default(false),
  createDatabase: z.boolean().default(false)
});

export const DeploymentController = {
  async list(ctx) {
    const deployments = await listDeploymentsForUser(ctx.user.id);
    const normalized = deployments.map((item) => ({
      ...item,
      domains: typeof item.domains === 'string' ? JSON.parse(item.domains) : item.domains,
      versions: typeof item.versions === 'string' ? JSON.parse(item.versions) : item.versions
    }));
    return ctx.json({ deployments: normalized });
  },

  async create(ctx) {
    const body = await ctx.body;
    const parsed = createDeploymentSchema.safeParse(body);
    if (!parsed.success) {
      return ctx.json({ error: 'Validation failed', details: parsed.error.flatten() }, 422);
    }
    const data = parsed.data;

    const deployment = await createDeployment({
      ownerId: ctx.user.id,
      name: data.name,
      repository: data.repository,
      branch: data.branch,
      blueGreenEnabled: data.enableBlueGreen
    });

    for (const [key, value] of Object.entries(data.environment)) {
      await upsertEnvVar(deployment.id, key, value);
    }

    for (const hostname of data.domains) {
      await addDomain({ deploymentId: deployment.id, hostname });
    }

    if (data.createDatabase) {
      const db = await provisionDatabase(deployment.id);
      await createDatabaseRecord({
        deploymentId: deployment.id,
        name: db.name,
        status: 'ready',
        connectionUrl: db.connectionUrl
      });
      await upsertEnvVar(deployment.id, 'DATABASE_URL', db.connectionUrl);
    }

    await createTask('deploy', { deploymentId: deployment.id });

    return ctx.json({ deployment });
  },

  async detail(ctx) {
    const deployment = await findDeploymentById(ctx.params.id);
    if (!deployment) {
      return ctx.json({ error: 'Not found' }, 404);
    }
    const [domains, envVars, versions, databases, logs, tasks] = await Promise.all([
      listDomains(deployment.id),
      listEnvVars(deployment.id),
      listVersions(deployment.id),
      listDatabasesForDeployment(deployment.id),
      listDeploymentLogs(deployment.id, 200),
      listRecentTasks(20)
    ]);
    const normalizedDeployment = {
      ...deployment,
      domains: typeof deployment.domains === 'string' ? JSON.parse(deployment.domains) : deployment.domains
    };

    return ctx.json({
      deployment: normalizedDeployment,
      domains,
      environment: envVars,
      versions,
      databases,
      logs,
      tasks: tasks.filter((task) => task.payload && task.payload.deploymentId === deployment.id)
    });
  },

  async updateEnv(ctx) {
    const deployment = await findDeploymentById(ctx.params.id);
    if (!deployment) {
      return ctx.json({ error: 'Deployment not found' }, 404);
    }
    const body = await ctx.body;
    const { key, value } = body;
    if (!key || typeof value !== 'string') {
      return ctx.json({ error: 'Invalid payload' }, 422);
    }
    await upsertEnvVar(deployment.id, key, value);
    return ctx.json({ ok: true });
  },

  async deleteEnv(ctx) {
    const deployment = await findDeploymentById(ctx.params.id);
    if (!deployment) {
      return ctx.json({ error: 'Deployment not found' }, 404);
    }
    const body = await ctx.body;
    const { key } = body;
    if (!key) {
      return ctx.json({ error: 'Key required' }, 422);
    }
    await removeEnvVar(deployment.id, key);
    return ctx.json({ ok: true });
  },

  async addDomain(ctx) {
    const deployment = await findDeploymentById(ctx.params.id);
    if (!deployment) {
      return ctx.json({ error: 'Deployment not found' }, 404);
    }
    const body = await ctx.body;
    const { hostname } = body;
    if (!hostname) {
      return ctx.json({ error: 'Hostname required' }, 422);
    }
    const domain = await addDomain({ deploymentId: deployment.id, hostname });
    return ctx.json({ domain }, 201);
  },

  async removeDomain(ctx) {
    const body = await ctx.body;
    const { domainId } = body;
    if (!domainId) {
      return ctx.json({ error: 'domainId required' }, 422);
    }
    await deleteDomain(domainId);
    return ctx.json({ ok: true });
  },

  async triggerDeploy(ctx) {
    const deployment = await findDeploymentById(ctx.params.id);
    if (!deployment) {
      return ctx.json({ error: 'Deployment not found' }, 404);
    }
    await createTask('deploy', { deploymentId: deployment.id });
    await recordDeploymentLog(deployment.id, 'info', 'Deployment manually triggered', {
      userId: ctx.user.id
    });
    return ctx.json({ ok: true });
  },

  async restart(ctx) {
    const deployment = await findDeploymentById(ctx.params.id);
    if (!deployment) {
      return ctx.json({ error: 'Deployment not found' }, 404);
    }
    await createTask('restart', { deploymentId: deployment.id });
    return ctx.json({ ok: true });
  },

  async rollback(ctx) {
    const deployment = await findDeploymentById(ctx.params.id);
    if (!deployment) {
      return ctx.json({ error: 'Deployment not found' }, 404);
    }
    const body = await ctx.body;
    const { versionId } = body;
    if (!versionId) {
      return ctx.json({ error: 'versionId required' }, 422);
    }
    const versions = await listVersions(deployment.id);
    const target = versions.find((v) => v.id === versionId);
    if (!target) {
      return ctx.json({ error: 'Version not found' }, 404);
    }
    await createTask('rollback', { deploymentId: deployment.id, version: target });
    await recordDeploymentLog(deployment.id, 'warn', 'Rollback requested', {
      versionId
    });
    return ctx.json({ ok: true });
  },

  async destroy(ctx) {
    const deployment = await findDeploymentById(ctx.params.id);
    if (!deployment) {
      return ctx.json({ error: 'Deployment not found' }, 404);
    }
    const databases = await listDatabasesForDeployment(deployment.id);
    for (const db of databases) {
      await dropDatabase(db.name);
      await deleteDatabase(db.id);
    }
    await recordDeploymentLog(deployment.id, 'warn', 'Deployment marked for deletion', {});
    await createTask('cleanup', { deploymentId: deployment.id });
    return ctx.json({ ok: true });
  },
  
  async streamLogs(ctx) {
    const deployment = await findDeploymentById(ctx.params.id);
    if (!deployment) {
      return ctx.json({ error: 'Deployment not found' }, 404);
    }
    let intervalId = null;
    const stream = new ReadableStream({
      start(controller) {
        let lastTimestamp = null;
        const sendLogs = async () => {
          const logs = await listDeploymentLogs(deployment.id, 50);
          const filtered = logs
            .filter((log) => {
              if (!lastTimestamp) return true;
              const created = new Date(log.created_at).getTime();
              return created > lastTimestamp;
            })
            .reverse();
          if (logs.length) {
            lastTimestamp = new Date(logs[0].created_at).getTime();
          }
          filtered.forEach((log) => {
            controller.enqueue(encoder.encode(`event: log\ndata: ${JSON.stringify(log)}\n\n`));
          });
        };
        intervalId = setInterval(() => {
          sendLogs().catch(() => {});
        }, 4000);
        sendLogs().catch(() => {});
        controller.enqueue(encoder.encode('event: ready\ndata: {}\n\n'));
      },
      cancel() {
        if (intervalId) clearInterval(intervalId);
      }
    });
    return new Response(stream, {
      headers: {
        'content-type': 'text/event-stream',
        'cache-control': 'no-cache',
        connection: 'keep-alive'
      }
    });
  }
};
