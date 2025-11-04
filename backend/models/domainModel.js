import { nanoid } from 'nanoid';
import { query, single } from '../lib/db.js';

export async function addDomain({ deploymentId, hostname }) {
  const id = nanoid();
  await query(
    `
      INSERT INTO deployment_domains (id, deployment_id, hostname)
      VALUES ($1, $2, $3)
    `,
    [id, deploymentId, hostname]
  );
  return findDomainById(id);
}

export async function findDomainById(id) {
  return single('SELECT * FROM deployment_domains WHERE id = $1', [id]);
}

export async function listDomains(deploymentId) {
  return query(
    `
      SELECT *
      FROM deployment_domains
      WHERE deployment_id = $1
      ORDER BY created_at DESC
    `,
    [deploymentId]
  );
}

export async function updateDomain(id, updates) {
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
      UPDATE deployment_domains
      SET ${fields.join(', ')}, updated_at = NOW()
      WHERE id = $${index}
    `,
    values
  );
  return findDomainById(id);
}

export async function deleteDomain(id) {
  await query('DELETE FROM deployment_domains WHERE id = $1', [id]);
}
