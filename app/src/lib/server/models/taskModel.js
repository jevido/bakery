import { nanoid } from 'nanoid';
import { sql } from 'bun';

export async function createTask(type, payload = {}, options = {}) {
	const id = nanoid();
	const { nodeId = null } = options;
	await sql`
    INSERT INTO tasks (id, type, payload, status, node_id)
    VALUES (${id}, ${type}, ${JSON.stringify(payload)}::jsonb, 'pending', ${nodeId})
  `;
	return findTaskById(id);
}

export async function reservePendingTask(nodeId = null, reservedBy = null) {
	const rows = await sql`
    UPDATE tasks
    SET status = 'running', started_at = NOW(), reserved_by = ${reservedBy}
    WHERE id = (
      SELECT id
      FROM tasks
      WHERE status = 'pending'
      AND (
        ${nodeId}::text IS NULL
        OR node_id = ${nodeId}
      )
      ORDER BY created_at
      FOR UPDATE SKIP LOCKED
      LIMIT 1
    )
    RETURNING *
  `;
	const row = rows[0];
	if (!row) return null;
	return {
		...row,
		payload: typeof row.payload === 'string' ? JSON.parse(row.payload) : row.payload
	};
}

export async function finishTask(id, status, error = null) {
	await sql`
    UPDATE tasks
    SET status = ${status},
        error = ${error},
        finished_at = NOW(),
        reserved_by = NULL
    WHERE id = ${id}
  `;
}

export async function findTaskById(id) {
	const rows = await sql`SELECT * FROM tasks WHERE id = ${id}`;
	const row = rows[0];
	if (!row) return null;
	return {
		...row,
		payload: typeof row.payload === 'string' ? JSON.parse(row.payload) : row.payload
	};
}

export async function listRecentTasks(limit = 50) {
	const rows = await sql`
    SELECT * FROM tasks
    ORDER BY created_at DESC
    LIMIT ${limit}
  `;
	return rows.map((row) => ({
		...row,
		payload: typeof row.payload === 'string' ? JSON.parse(row.payload) : row.payload
	}));
}

export async function hasActiveDeployTask(deploymentId) {
	const rows = await sql`
	  SELECT 1
	  FROM tasks
	  WHERE type = 'deploy'
	    AND status IN ('pending', 'running')
	    AND payload ->> 'deploymentId' = ${deploymentId}
	  LIMIT 1
	`;
	return rows.length > 0;
}
