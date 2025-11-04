import { nanoid } from 'nanoid';
import { sql } from '../lib/db.js';

export async function addDomain({ deploymentId, hostname }) {
  const id = nanoid();
  await sql`
    INSERT INTO deployment_domains (id, deployment_id, hostname)
    VALUES (${id}, ${deploymentId}, ${hostname})
  `;
  return findDomainById(id);
}

export async function findDomainById(id) {
  const rows = await sql`SELECT * FROM deployment_domains WHERE id = ${id}`;
  return rows[0] ?? null;
}

export async function listDomains(deploymentId) {
  return sql`
    SELECT *
    FROM deployment_domains
    WHERE deployment_id = ${deploymentId}
    ORDER BY created_at DESC
  `;
}

export async function updateDomain(id, updates) {
  const rows = await sql`
    UPDATE deployment_domains
    SET ${sql(updates)}, updated_at = NOW()
    WHERE id = ${id}
    RETURNING *
  `;
  return rows[0] ?? null;
}

export async function deleteDomain(id) {
  await sql`DELETE FROM deployment_domains WHERE id = ${id}`;
}
