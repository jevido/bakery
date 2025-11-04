import { nanoid } from 'nanoid';
import { query, single } from '../lib/db.js';

export async function createDatabaseRecord({
  deploymentId,
  name,
  status,
  connectionUrl
}) {
  const id = nanoid();
  await query(
    `
      INSERT INTO deployment_databases (
        id, deployment_id, name, status, connection_url
      )
      VALUES ($1, $2, $3, $4, $5)
    `,
    [id, deploymentId, name, status, connectionUrl]
  );
  return findDatabaseById(id);
}

export async function findDatabaseById(id) {
  return single('SELECT * FROM deployment_databases WHERE id = $1', [id]);
}

export async function listDatabasesForDeployment(deploymentId) {
  return query(
    `
      SELECT *
      FROM deployment_databases
      WHERE deployment_id = $1
      ORDER BY created_at DESC
    `,
    [deploymentId]
  );
}

export async function updateDatabase(id, updates) {
  const fields = [];
  const values = [];
  let index = 1;
  Object.entries(updates).forEach(([key, value]) => {
    fields.push(`${key} = $${index++}`);
    values.push(value);
  });
  values.push(id);
  await query(
    `
      UPDATE deployment_databases
      SET ${fields.join(', ')}, updated_at = NOW()
      WHERE id = $${index}
    `,
    values
  );
  return findDatabaseById(id);
}

export async function deleteDatabase(id) {
  await query('DELETE FROM deployment_databases WHERE id = $1', [id]);
}
