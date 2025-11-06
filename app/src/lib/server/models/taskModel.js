import { nanoid } from 'nanoid';
import { sql } from 'bun';

export async function createTask(type, payload = {}) {
	const id = nanoid();
	await sql`
    INSERT INTO tasks (id, type, payload, status)
    VALUES (${id}, ${type}, ${JSON.stringify(payload)}::jsonb, 'pending')
  `;
	return findTaskById(id);
}

export async function reservePendingTask() {
	const rows = await sql`
    UPDATE tasks
    SET status = 'running', started_at = NOW()
    WHERE id = (
      SELECT id
      FROM tasks
      WHERE status = 'pending'
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
        finished_at = NOW()
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
