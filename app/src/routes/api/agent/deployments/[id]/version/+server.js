import { json, error } from '@sveltejs/kit';
import { z } from 'zod';
import { requireAgent } from '$lib/server/agentAuth.js';
import { findDeploymentById, recordDeploymentVersion } from '$lib/server/models/deploymentModel.js';

const schema = z.object({
	slot: z.enum(['blue', 'green']),
	commitSha: z.string().optional(),
	status: z.enum(['active', 'inactive']).default('active'),
	port: z.number().int().optional(),
	dockerized: z.boolean().optional(),
	artifactPath: z.string().optional()
});

export const POST = async ({ params, request }) => {
	const node = await requireAgent(request);
	const deployment = await findDeploymentById(params.id);
	if (!deployment || deployment.node_id !== node.id) {
		throw error(404, 'Deployment not found');
	}
	const body = await request.json().catch(() => ({}));
	const parsed = schema.safeParse(body);
	if (!parsed.success) {
		throw error(422, 'Invalid payload');
	}
	const versionId = await recordDeploymentVersion({
		deploymentId: deployment.id,
		slot: parsed.data.slot,
		commitSha: parsed.data.commitSha,
		status: parsed.data.status,
		port: parsed.data.port,
		dockerized: parsed.data.dockerized ?? deployment.dockerized,
		artifactPath: parsed.data.artifactPath ?? null
	});
	return json({ versionId });
};
