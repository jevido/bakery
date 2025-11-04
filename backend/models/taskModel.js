import { nanoid } from 'nanoid';
import { query, single } from '../lib/db.js';

export async function createTask(type, payload = {}) {
  const id = nanoid();
  await query(
    `
      INSERT INTO tasks (id, type, payload, status)
      VALUES ($1, $2, $3::jsonb, 'pending')
    `,
    [id, type, JSON.stringify(payload)]
  );
  return findTaskById(id);
}

export async function reservePendingTask() {
  const row = await single(
    `
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
    `
  );
  if (!row) return null;
  return {
    ...row,
    payload: typeof row.payload === 'string' ? JSON.parse(row.payload) : row.payload
  };
}

export async function finishTask(id, status, error = null) {
  await query(
    `
      UPDATE tasks
      SET status = $2,
          error = $3,
          finished_at = NOW()
      WHERE id = $1
    `,
    [id, status, error]
  );
}

export async function findTaskById(id) {
  const row = await single('SELECT * FROM tasks WHERE id = $1', [id]);
  if (!row) return null;
  return {
    ...row,
    payload: typeof row.payload === 'string' ? JSON.parse(row.payload) : row.payload
  };
}

export async function listRecentTasks(limit = 50) {
  const rows = await query(
    `
      SELECT * FROM tasks
      ORDER BY created_at DESC
      LIMIT $1
    `,
    [limit]
  );
  return rows.map((row) => ({
    ...row,
    payload: typeof row.payload === 'string' ? JSON.parse(row.payload) : row.payload
  }));
}
