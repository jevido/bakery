import { nanoid } from 'nanoid';
import { sql } from '../lib/db.js';

export async function createDatabaseRecord({
  deploymentId,
  name,
  status,
  connectionUrl
}) {
  const id = nanoid();
  await sql`
    INSERT INTO deployment_databases (
      id, deployment_id, name, status, connection_url
    )
    VALUES (${id}, ${deploymentId}, ${name}, ${status}, ${connectionUrl})
  `;
  return findDatabaseById(id);
}

export async function findDatabaseById(id) {
  const rows = await sql`SELECT * FROM deployment_databases WHERE id = ${id}`;
  return rows[0] ?? null;
}

export async function listDatabasesForDeployment(deploymentId) {
  return sql`
    SELECT *
    FROM deployment_databases
    WHERE deployment_id = ${deploymentId}
    ORDER BY created_at DESC
  `;
}

export async function updateDatabase(id, updates) {
  const rows = await sql`
    UPDATE deployment_databases
    SET ${sql(updates)}, updated_at = NOW()
    WHERE id = ${id}
    RETURNING *
  `;
  return rows[0] ?? null;
}

export async function deleteDatabase(id) {
  await sql`DELETE FROM deployment_databases WHERE id = ${id}`;
}
