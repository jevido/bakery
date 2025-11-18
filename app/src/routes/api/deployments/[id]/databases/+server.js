import { json, error } from '@sveltejs/kit';
import { findDeploymentById } from '$lib/server/models/deploymentModel.js';
import {
	listDatabasesForDeployment,
	deleteDatabase,
	findDatabaseById,
	createDatabaseRecord
} from '$lib/server/models/databaseModel.js';
import {
	provisionDatabase,
	provisionDatabaseOnNode,
	dropDatabase,
	dropDatabaseOnNode,
	parseDatabaseUrl
} from '$lib/server/postgresAdmin.js';
import { getNodeWithCredentials } from '$lib/server/models/nodeModel.js';
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
	const node = deployment.node_id ? await getNodeWithCredentials(deployment.node_id) : null;
	const db = node
		? await provisionDatabaseOnNode(node, deployment.id)
		: await provisionDatabase(deployment.id);
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
	const record = await findDatabaseById(databaseId);
	const node = deployment.node_id ? await getNodeWithCredentials(deployment.node_id) : null;
	if (node && record) {
		const parsed = record.connection_url ? parseDatabaseUrl(record.connection_url) : null;
		await dropDatabaseOnNode(node, name, parsed?.user);
	} else {
		await dropDatabase(name);
	}
	await deleteDatabase(databaseId);
	return json({ ok: true });
};
