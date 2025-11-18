import { json, error } from '@sveltejs/kit';
import { Buffer } from 'node:buffer';
import { z } from 'zod';
import {
	getNodeWithCredentials,
	updateNode,
	markNodeActive
} from '$lib/server/models/nodeModel.js';
import { runSshCommand } from '$lib/server/sshClient.js';

const schema = z.object({
	host: z.string().trim().min(1).optional(),
	port: z.number().int().min(1).max(65535).optional(),
	user: z.string().trim().min(1).optional()
});

export const POST = async ({ params, locals, request }) => {
	if (!locals.user) {
		throw error(401, 'Unauthorized');
	}
	const node = await getNodeWithCredentials(params.id);
	if (!node || node.owner_id !== locals.user.id) {
		throw error(404, 'Node not found');
	}

	const body = await request.json().catch(() => ({}));
	const parsed = schema.safeParse(body);
	if (!parsed.success) {
		throw error(422, 'Invalid payload');
	}

	const updates = {};
	if (parsed.data.host) updates.ssh_host = parsed.data.host;
	if (parsed.data.port) updates.ssh_port = parsed.data.port;
	if (parsed.data.user) updates.ssh_user = parsed.data.user;

	let targetNode = node;
	if (Object.keys(updates).length) {
		targetNode = await updateNode(node.id, updates);
		targetNode = {
			...targetNode,
			ssh_private_key: node.ssh_private_key,
			ssh_public_key: node.ssh_public_key
		};
	}

	if (!targetNode.ssh_host) {
		throw error(400, 'Provide an SSH host before verifying');
	}

	await runSshCommand(
		{
			host: targetNode.ssh_host,
			port: targetNode.ssh_port || 22,
			user: targetNode.ssh_user || 'bakery-agent',
			privateKey: node.ssh_private_key
		},
		'echo bakery-ready'
	);

	const updated = await markNodeActive(node.id);
	const finalNode = await updateNode(node.id, {
		ssh_last_connected: new Date(),
		last_seen: new Date()
	});
	return json({ node: sanitize(finalNode) });
};

const INSTALLER_URL =
	'https://raw.githubusercontent.com/jevido/bakery/main/scripts/install-node-agent.sh';
const DEFAULT_SSH_USER = 'bakery-agent';

function buildInstallCommand(node) {
	if (!node?.ssh_public_key) return null;
	const keyBase64 = Buffer.from(node.ssh_public_key, 'utf8').toString('base64');
	const sshUser = node.ssh_user || DEFAULT_SSH_USER;
	return `curl -fsSL ${INSTALLER_URL} | sudo SSH_USER=${sshUser} SSH_KEY_BASE64=${keyBase64} bash`;
}

function sanitize(node) {
	if (!node) return node;
	const {
		api_token: _apiToken,
		install_token: _installToken,
		pairing_code: _pairing,
		ssh_private_key: _priv,
		...rest
	} = node;
	const installCommand = buildInstallCommand(node);
	return {
		...rest,
		install_command: node.status === 'active' ? null : installCommand,
		update_command: installCommand
	};
}
