import { nanoid } from 'nanoid';
import { sql } from 'bun';

function mapRow(row) {
	if (!row) return null;
	return {
		...row,
		metadata: typeof row.metadata === 'string' ? JSON.parse(row.metadata) : row.metadata
	};
}

export async function createNode({ ownerId, name }) {
	const id = nanoid();
	const installToken = nanoid(32);
	const rows = await sql`
    INSERT INTO nodes (id, owner_id, name, status, install_token)
    VALUES (${id}, ${ownerId}, ${name}, 'pending', ${installToken})
    RETURNING *
  `;
	const node = mapRow(rows[0]);
	return { node, installToken };
}

export async function listNodes(ownerId) {
	const rows = await sql`
    SELECT *
    FROM nodes
    WHERE owner_id = ${ownerId}
    ORDER BY created_at DESC
  `;
	return rows.map(mapRow);
}

export async function findNodeById(id) {
	const rows = await sql`SELECT * FROM nodes WHERE id = ${id}`;
	return mapRow(rows[0]);
}

export async function updateNode(id, updates) {
	const rows = await sql`
    UPDATE nodes
    SET ${sql(updates)}, updated_at = NOW()
    WHERE id = ${id}
    RETURNING *
  `;
	return mapRow(rows[0]);
}

export async function rotateInstallToken(id) {
	const installToken = nanoid(32);
	const rows = await sql`
    UPDATE nodes
    SET install_token = ${installToken}, updated_at = NOW()
    WHERE id = ${id}
    RETURNING *
  `;
	return { node: mapRow(rows[0]), installToken };
}

export async function consumeInstallToken(token, metadata = {}) {
	const rows = await sql`
    SELECT * FROM nodes WHERE install_token = ${token}
  `;
	const row = rows[0];
	if (!row) return null;
	const apiToken = nanoid(48);
	const pairingCode = nanoid(12).toUpperCase();
	const updatedRows = await sql`
    UPDATE nodes
    SET install_token = NULL,
        api_token = ${apiToken},
        pairing_code = ${pairingCode},
        status = 'awaiting_pairing',
        metadata = COALESCE(metadata, '{}'::jsonb) || ${JSON.stringify(metadata)}::jsonb,
        last_seen = NOW(),
        updated_at = NOW()
    WHERE id = ${row.id}
    RETURNING *
  `;
	return {
		node: mapRow(updatedRows[0]),
		apiToken,
		pairingCode
	};
}

export async function authenticateAgent(apiToken) {
	const rows = await sql`
    SELECT * FROM nodes WHERE api_token = ${apiToken}
  `;
	return mapRow(rows[0]);
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

export async function markNodeActive(id, pairingCode) {
	const rows = await sql`
    UPDATE nodes
    SET status = 'active', pairing_code = NULL, updated_at = NOW()
    WHERE id = ${id} AND pairing_code = ${pairingCode}
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
    DELETE FROM nodes
    WHERE id = ${id} AND owner_id = ${ownerId}
  `;
}
