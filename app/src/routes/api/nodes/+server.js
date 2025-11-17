import { json, error } from '@sveltejs/kit';
import { Buffer } from 'node:buffer';
import { z } from 'zod';
import { listNodes, createNode, deleteNode } from '$lib/server/models/nodeModel.js';

const createNodeSchema = z.object({
	name: z.string().min(3)
});

const INSTALLER_URL =
	'https://raw.githubusercontent.com/jevido/bakery/main/scripts/install-node-agent.sh';
const DEFAULT_SSH_USER = 'bakery-agent';

function buildUpdateCommand(node) {
	if (!node?.ssh_public_key) return null;
	const keyBase64 = Buffer.from(node.ssh_public_key, 'utf8').toString('base64');
	const sshUser = node.ssh_user || DEFAULT_SSH_USER;
	return `curl -fsSL ${INSTALLER_URL} | sudo SSH_USER=${sshUser} SSH_KEY_BASE64=${keyBase64} bash`;
}

function buildInstallCommand(node) {
	return buildUpdateCommand(node);
}

function sanitizeNode(node) {
	if (!node) return node;
	const {
		api_token: _apiToken,
		install_token: _installToken,
		pairing_code: _pairingCode,
		ssh_private_key: _sshPrivateKey,
		...rest
	} = node;
	return {
		...rest,
		install_command: node.status === 'active' ? null : buildInstallCommand(node),
		update_command: buildUpdateCommand(node)
	};
}

export const GET = async ({ locals }) => {
	if (!locals.user) {
		throw error(401, 'Unauthorized');
	}
	const nodes = await listNodes(locals.user.id);
	return json({
		controlPlane: {
			id: 'control-plane',
			name: 'Control plane',
			status: 'active'
		},
		nodes: nodes.map(sanitizeNode),
		installBlocked: false,
		installWarning: null,
		apiBase: null
	});
};

export const POST = async ({ request, locals }) => {
	if (!locals.user) {
		throw error(401, 'Unauthorized');
	}
	const body = await request.json().catch(() => ({}));
	const parsed = createNodeSchema.safeParse(body);
	if (!parsed.success) {
		throw error(422, 'Validation failed');
	}
	const node = await createNode({
		ownerId: locals.user.id,
		name: parsed.data.name
	});
	return json({
		node: sanitizeNode(node)
	});
};

export const DELETE = async ({ url, locals }) => {
	if (!locals.user) {
		throw error(401, 'Unauthorized');
	}
	const id = url.searchParams.get('id');
	if (!id) {
		throw error(422, 'id required');
	}
	await deleteNode(id, locals.user.id);
	return json({ ok: true });
};
