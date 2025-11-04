import { nanoid } from 'nanoid';
import { query, single } from '../lib/db.js';

export async function listDeploymentsForUser(userId) {
  return query(
    `
      SELECT d.*,
             COALESCE(json_agg(DISTINCT dd.*)
               FILTER (WHERE dd.id IS NOT NULL), '[]') AS domains,
             COALESCE(
               json_agg(DISTINCT dv.* ORDER BY dv.created_at DESC)
               FILTER (WHERE dv.id IS NOT NULL), '[]'
             ) AS versions
      FROM deployments d
      LEFT JOIN deployment_domains dd ON dd.deployment_id = d.id
      LEFT JOIN deployment_versions dv ON dv.deployment_id = d.id
      WHERE d.owner_id = $1
      GROUP BY d.id
      ORDER BY d.updated_at DESC
    `,
    [userId]
  );
}

export async function findDeploymentById(id) {
  return single(
    `
      SELECT d.*,
             COALESCE(
               json_agg(DISTINCT dd.*)
               FILTER (WHERE dd.id IS NOT NULL),
               '[]'
             ) AS domains
      FROM deployments d
      LEFT JOIN deployment_domains dd ON dd.deployment_id = d.id
      WHERE d.id = $1
      GROUP BY d.id
    `,
    [id]
  );
}

export async function createDeployment(payload) {
  const id = nanoid();
  const {
    ownerId,
    name,
    repository,
    branch,
    blueGreenEnabled,
    dockerized = false
  } = payload;
  await query(
    `
      INSERT INTO deployments (
        id, owner_id, name, repository, branch,
        blue_green_enabled, dockerized, status
      ) VALUES ($1, $2, $3, $4, $5, $6, $7, 'pending')
    `,
    [id, ownerId, name, repository, branch, blueGreenEnabled, dockerized]
  );
  return findDeploymentById(id);
}

export async function updateDeployment(id, updates) {
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
      UPDATE deployments
      SET ${fields.join(', ')}, updated_at = NOW()
      WHERE id = $${index}
    `,
    values
  );
  return findDeploymentById(id);
}

export async function recordDeploymentVersion(payload) {
  const id = nanoid();
  const {
    deploymentId,
    slot,
    commitSha,
    status,
    port,
    dockerized,
    artifactPath
  } = payload;
  await query(
    `
      UPDATE deployment_versions
      SET status = 'inactive'
      WHERE deployment_id = $1 AND slot = $2
    `,
    [deploymentId, slot]
  );
  await query(
    `
      INSERT INTO deployment_versions (
        id, deployment_id, slot, commit_sha, status, port,
        dockerized, artifact_path
      ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
    `,
    [id, deploymentId, slot, commitSha, status, port, dockerized, artifactPath]
  );
  return id;
}

export async function getActiveVersion(deploymentId) {
  return single(
    `
      SELECT *
      FROM deployment_versions
      WHERE deployment_id = $1 AND status = 'active'
      ORDER BY created_at DESC
      LIMIT 1
    `,
    [deploymentId]
  );
}

export async function listVersions(deploymentId) {
  return query(
    `
      SELECT *
      FROM deployment_versions
      WHERE deployment_id = $1
      ORDER BY created_at DESC
      LIMIT 20
    `,
    [deploymentId]
  );
}

export async function recordDeploymentLog(deploymentId, level, message, meta = {}) {
  await query(
    `
      INSERT INTO deployment_logs (id, deployment_id, level, message, metadata)
      VALUES ($1, $2, $3, $4, $5::jsonb)
    `,
    [nanoid(), deploymentId, level, message, JSON.stringify(meta)]
  );
}

export async function listDeploymentLogs(deploymentId, limit = 200) {
  return query(
    `
      SELECT *
      FROM deployment_logs
      WHERE deployment_id = $1
      ORDER BY created_at DESC
      LIMIT $2
    `,
    [deploymentId, limit]
  );
}

export async function deleteDeployment(id) {
  await query('DELETE FROM deployments WHERE id = $1', [id]);
}
