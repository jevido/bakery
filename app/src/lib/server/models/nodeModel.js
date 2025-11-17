import { nanoid } from 'nanoid';
import { sql } from 'bun';
import { encrypt, decrypt } from '../crypto.js';
import { mkdtemp, readFile, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';

function mapRow(row) {
	if (!row) return null;
	const { ssh_private_key: _private, ...rest } = row;
	return {
		...rest,
		metadata: typeof rest.metadata === 'string' ? JSON.parse(rest.metadata) : rest.metadata
	};
}

async function generateSshKeyPair(comment) {
	const dir = await mkdtemp(join(tmpdir(), 'bakery-ssh-'));
	const keyPath = join(dir, 'id_ed25519');
	const child = Bun.spawn([
		'ssh-keygen',
		'-q',
		'-t',
		'ed25519',
		'-N',
		'',
		'-C',
		comment,
		'-f',
		keyPath
	]);
	const exitCode = await child.exited;
	if (exitCode !== 0) {
		await rm(dir, { recursive: true, force: true });
		throw new Error('Failed to generate SSH key pair. Ensure ssh-keygen is installed.');
	}
	const privateKey = await readFile(keyPath, 'utf8');
	const publicKey = await readFile(`${keyPath}.pub`, 'utf8');
	await rm(dir, { recursive: true, force: true });
	return { privateKey, publicKey };
}

export async function ensureNodeSshKeys(id, row) {
	let privateKey = row?.ssh_private_key ? decrypt(row.ssh_private_key) : null;
	let publicKey = row?.ssh_public_key || null;
	if (privateKey && publicKey) {
		return { privateKey, publicKey, row };
	}
	const pair = await generateSshKeyPair(`bakery-node-${id}`);
	privateKey = pair.privateKey;
	publicKey = pair.publicKey;
	await sql`
    UPDATE nodes
    SET ssh_private_key = ${encrypt(privateKey)},
        ssh_public_key = ${publicKey},
        updated_at = NOW()
    WHERE id = ${id}
  `;
	const refreshed = await sql`SELECT * FROM nodes WHERE id = ${id}`;
	return { privateKey, publicKey, row: refreshed[0] };
}

export async function createNode({ ownerId, name }) {
	const id = nanoid();
	const rows = await sql`
    INSERT INTO nodes (id, owner_id, name, status)
    VALUES (${id}, ${ownerId}, ${name}, 'pending')
    RETURNING *
  `;
	const { row: refreshed } = await ensureNodeSshKeys(id, rows[0]);
	return mapRow(refreshed ?? rows[0]);
}

export async function listNodes(ownerId) {
	const rows = await sql`
    SELECT *
    FROM nodes
    WHERE owner_id = ${ownerId}
    ORDER BY created_at DESC
  `;
	return rows.map((row) => mapRow(row));
}

export async function findNodeById(id) {
	const rows = await sql`SELECT * FROM nodes WHERE id = ${id}`;
	return mapRow(rows[0]);
}

export async function updateNode(id, updates) {
	if (typeof updates.ssh_private_key === 'string' && updates.ssh_private_key.length) {
		updates.ssh_private_key = encrypt(updates.ssh_private_key);
	}
	const rows = await sql`
    UPDATE nodes
    SET ${sql(updates)}, updated_at = NOW()
    WHERE id = ${id}
    RETURNING *
  `;
	return mapRow(rows[0]);
}

export async function rotateInstallToken() {
	return null;
}

export async function consumeInstallToken() {
	return null;
}

export async function authenticateAgent() {
	return null;
}

export async function setNodeStatus(id, status) {
	const rows = await sql`
    UPDATE nodes
    SET status = ${status}, updated_at = NOW()
    WHERE id = ${id}
    RETURNING *
  `;
	return mapRow(rows[0]);
}

export async function markNodeActive(id) {
	const rows = await sql`
    UPDATE nodes
    SET status = 'active', pairing_code = NULL, updated_at = NOW()
    WHERE id = ${id}
    RETURNING *
  `;
	return mapRow(rows[0]);
}

export async function touchNode(id, metadata = {}) {
	const rows = await sql`
    UPDATE nodes
    SET last_seen = NOW(),
        metadata = COALESCE(metadata, '{}'::jsonb) || ${JSON.stringify(metadata)}::jsonb,
        updated_at = NOW()
    WHERE id = ${id}
    RETURNING *
  `;
	return mapRow(rows[0]);
}

export async function deleteNode(id, ownerId) {
	await sql`
    UPDATE tasks
    SET node_id = NULL
    WHERE node_id = ${id}
  `;
	await sql`
    UPDATE deployments
    SET node_id = NULL
    WHERE node_id = ${id} AND owner_id = ${ownerId}
  `;
	await sql`
    DELETE FROM nodes
    WHERE id = ${id} AND owner_id = ${ownerId}
  `;
}

export async function getNodeWithCredentials(id) {
	const rows = await sql`
    SELECT *
    FROM nodes
    WHERE id = ${id}
  `;
	const row = rows[0];
	if (!row) return null;
	const { privateKey, publicKey, row: refreshed } = await ensureNodeSshKeys(id, row);
	return {
		...mapRow(refreshed ?? row),
		ssh_private_key: privateKey,
		ssh_public_key: publicKey
	};
}
