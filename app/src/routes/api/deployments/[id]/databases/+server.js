import { json, error } from '@sveltejs/kit';
import { findDeploymentById } from '$lib/server/models/deploymentModel.js';
import { listDatabasesForDeployment, deleteDatabase } from '$lib/server/models/databaseModel.js';
import { provisionDatabase, dropDatabase } from '$lib/server/postgresAdmin.js';
import { createDatabaseRecord } from '$lib/server/models/databaseModel.js';
import { upsertEnvVar } from '$lib/server/models/envModel.js';

export const GET = async ({ params, locals }) => {
	if (!locals.user) {
		throw error(401, 'Unauthorized');
	}
	const deployment = await findDeploymentById(params.id);
	if (!deployment) {
		throw error(404, 'Deployment not found');
	}
	return json({ databases: await listDatabasesForDeployment(deployment.id) });
};

export const POST = async ({ params, locals }) => {
	if (!locals.user) {
		throw error(401, 'Unauthorized');
	}
	const deployment = await findDeploymentById(params.id);
	if (!deployment) {
		throw error(404, 'Deployment not found');
	}
	const db = await provisionDatabase(deployment.id);
	await createDatabaseRecord({
		deploymentId: deployment.id,
		name: db.name,
		status: 'ready',
		connectionUrl: db.connectionUrl
	});
	await upsertEnvVar(deployment.id, 'DATABASE_URL', db.connectionUrl);
	return json({ ok: true });
};

export const DELETE = async ({ params, locals, request }) => {
	if (!locals.user) {
		throw error(401, 'Unauthorized');
	}
	const deployment = await findDeploymentById(params.id);
	if (!deployment) {
		throw error(404, 'Deployment not found');
	}
	const body = await request.json().catch(() => ({}));
	const { databaseId, name } = body;
	if (!databaseId || !name) {
		throw error(422, 'databaseId and name required');
	}
	await dropDatabase(name);
	await deleteDatabase(databaseId);
	return json({ ok: true });
};
