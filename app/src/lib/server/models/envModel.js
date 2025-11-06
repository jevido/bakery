import { nanoid } from 'nanoid';
import { sql } from 'bun';
import { encrypt, decrypt } from '../crypto.js';

export async function upsertEnvVar(deploymentId, key, value) {
	const encrypted = encrypt(value);
	await sql`
    INSERT INTO environment_variables (id, deployment_id, key, value)
    VALUES (${nanoid()}, ${deploymentId}, ${key}, ${encrypted})
    ON CONFLICT (deployment_id, key) DO UPDATE
    SET value = EXCLUDED.value, updated_at = NOW()
  `;
}

export async function removeEnvVar(deploymentId, key) {
	await sql`
    DELETE FROM environment_variables
    WHERE deployment_id = ${deploymentId} AND key = ${key}
  `;
}

export async function listEnvVars(deploymentId, includeSecrets = false) {
	const rows = await sql`
    SELECT key, value
    FROM environment_variables
    WHERE deployment_id = ${deploymentId}
  `;
	return rows.map((row) => ({
		key: row.key,
		value: includeSecrets ? decrypt(row.value) : null,
		hasValue: true
	}));
}

export async function exportEnvVars(deploymentId) {
	const rows = await sql`
    SELECT key, value
    FROM environment_variables
    WHERE deployment_id = ${deploymentId}
  `;
	return rows.reduce((acc, row) => {
		acc[row.key] = decrypt(row.value);
		return acc;
	}, {});
}
