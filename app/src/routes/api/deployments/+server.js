import { json, error } from '@sveltejs/kit';
import { z } from 'zod';
import { listDeploymentsForUser, createDeployment } from '$lib/server/models/deploymentModel.js';
import { addDomain } from '$lib/server/models/domainModel.js';
import { upsertEnvVar } from '$lib/server/models/envModel.js';
import { createTask } from '$lib/server/models/taskModel.js';
import { provisionDatabase } from '$lib/server/postgresAdmin.js';
import { createDatabaseRecord } from '$lib/server/models/databaseModel.js';
import { getGithubAccount } from '$lib/server/models/userModel.js';
import { findNodeById } from '$lib/server/models/nodeModel.js';

const createDeploymentSchema = z.object({
	name: z.string().min(3),
	repository: z.string(),
	branch: z.string(),
	domains: z.array(z.string()).default([]),
	environment: z.record(z.string()).default({}),
	enableBlueGreen: z.boolean().default(false),
	createDatabase: z.boolean().default(false),
	nodeId: z.string().optional().nullable()
});

export const GET = async ({ locals }) => {
	if (!locals.user) {
		throw error(401, 'Unauthorized');
	}
	const deployments = await listDeploymentsForUser(locals.user.id);
	const normalized = deployments.map((item) => {
		const domains = typeof item.domains === 'string' ? JSON.parse(item.domains) : item.domains;
		const versions = typeof item.versions === 'string' ? JSON.parse(item.versions) : item.versions;
		const { node_name, node_status, ...rest } = item;
		return {
			...rest,
			domains,
			versions,
			node:
				rest.node_id && node_name
					? {
							id: rest.node_id,
							name: node_name,
							status: node_status
						}
					: null
		};
	});
	return json({ deployments: normalized });
};

export const POST = async ({ request, locals }) => {
	if (!locals.user) {
		throw error(401, 'Unauthorized');
	}
	const account = await getGithubAccount(locals.user.id);
	if (!account) {
		throw error(400, 'Link a GitHub account before creating deployments');
	}
	const body = await request.json().catch(() => ({}));
	const parsed = createDeploymentSchema.safeParse(body);
	if (!parsed.success) {
		throw error(422, 'Validation failed');
	}
	const data = parsed.data;

	let nodeId = data.nodeId ?? null;
	if (nodeId) {
		const node = await findNodeById(nodeId);
		if (!node || node.owner_id !== locals.user.id) {
			throw error(400, 'Invalid server node');
		}
		if (node.status !== 'active') {
			throw error(400, 'Node is not active');
		}
	}

	const deployment = await createDeployment({
		ownerId: locals.user.id,
		name: data.name,
		repository: data.repository,
		branch: data.branch,
		blueGreenEnabled: data.enableBlueGreen,
		nodeId
	});

	const { node_name, node_status, ...deploymentRest } = deployment ?? {};
	const normalizedDeployment = deployment
		? {
				...deploymentRest,
				node:
					deployment.node_id && node_name
						? {
								id: deployment.node_id,
								name: node_name,
								status: node_status
							}
						: null
			}
		: null;

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

	await createTask('deploy', { deploymentId: deployment.id }, { nodeId: deployment.node_id });

	return json({ deployment: normalizedDeployment });
};
