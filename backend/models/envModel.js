import { nanoid } from 'nanoid';
import { query, single } from '../lib/db.js';
import { encrypt, decrypt } from '../lib/crypto.js';

export async function upsertEnvVar(deploymentId, key, value) {
  const encrypted = encrypt(value);
  await query(
    `
      INSERT INTO environment_variables (id, deployment_id, key, value)
      VALUES ($1, $2, $3, $4)
      ON CONFLICT (deployment_id, key) DO UPDATE
      SET value = EXCLUDED.value, updated_at = NOW()
    `,
    [nanoid(), deploymentId, key, encrypted]
  );
}

export async function removeEnvVar(deploymentId, key) {
  await query(
    `
      DELETE FROM environment_variables
      WHERE deployment_id = $1 AND key = $2
    `,
    [deploymentId, key]
  );
}

export async function listEnvVars(deploymentId, includeSecrets = false) {
  const rows = await query(
    `
      SELECT key, value
      FROM environment_variables
      WHERE deployment_id = $1
    `,
    [deploymentId]
  );
  return rows.map((row) => ({
    key: row.key,
    value: includeSecrets ? decrypt(row.value) : null,
    hasValue: true
  }));
}

export async function exportEnvVars(deploymentId) {
  const rows = await query(
    `
      SELECT key, value
      FROM environment_variables
      WHERE deployment_id = $1
    `,
    [deploymentId]
  );
  return rows.reduce((acc, row) => {
    acc[row.key] = decrypt(row.value);
    return acc;
  }, {});
}
