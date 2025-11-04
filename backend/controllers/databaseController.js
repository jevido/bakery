import {
  listDatabasesForDeployment,
  createDatabaseRecord,
  updateDatabase,
  deleteDatabase
} from '../models/databaseModel.js';
import { findDeploymentById } from '../models/deploymentModel.js';
import { provisionDatabase, dropDatabase } from '../lib/postgresAdmin.js';
import { upsertEnvVar, removeEnvVar } from '../models/envModel.js';

export const DatabaseController = {
  async list(ctx) {
    const deployment = await findDeploymentById(ctx.params.id);
    if (!deployment) {
      return ctx.json({ error: 'Deployment not found' }, 404);
    }
    const databases = await listDatabasesForDeployment(deployment.id);
    return ctx.json({ databases });
  },

  async create(ctx) {
    const deployment = await findDeploymentById(ctx.params.id);
    if (!deployment) {
      return ctx.json({ error: 'Deployment not found' }, 404);
    }
    const provisioned = await provisionDatabase(deployment.id);
    const record = await createDatabaseRecord({
      deploymentId: deployment.id,
      name: provisioned.name,
      status: 'ready',
      connectionUrl: provisioned.connectionUrl
    });
    await upsertEnvVar(deployment.id, 'DATABASE_URL', provisioned.connectionUrl);
    return ctx.json({ database: record }, 201);
  },

  async remove(ctx) {
    const deployment = await findDeploymentById(ctx.params.id);
    if (!deployment) {
      return ctx.json({ error: 'Deployment not found' }, 404);
    }
    const body = await ctx.body;
    const { databaseId } = body;
    if (!databaseId) {
      return ctx.json({ error: 'databaseId required' }, 422);
    }
    const databases = await listDatabasesForDeployment(deployment.id);
    const target = databases.find((db) => db.id === databaseId);
    if (!target) {
      return ctx.json({ error: 'Database not found' }, 404);
    }
    await dropDatabase(target.name);
    await deleteDatabase(target.id);
    await removeEnvVar(deployment.id, 'DATABASE_URL');
    return ctx.json({ ok: true });
  }
};
