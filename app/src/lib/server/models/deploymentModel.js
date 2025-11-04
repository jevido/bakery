import { nanoid } from 'nanoid';
import { sql } from 'bun';

export async function listDeploymentsForUser(userId) {
  return sql`
    SELECT
      d.*,
      COALESCE(
        (
          SELECT json_agg(dd ORDER BY dd.created_at DESC)
          FROM deployment_domains dd
          WHERE dd.deployment_id = d.id
        ),
        '[]'::json
      ) AS domains,
      COALESCE(
        (
          SELECT json_agg(dv ORDER BY dv.created_at DESC)
          FROM deployment_versions dv
          WHERE dv.deployment_id = d.id
        ),
        '[]'::json
      ) AS versions
    FROM deployments d
    WHERE d.owner_id = ${userId}
    ORDER BY d.updated_at DESC
  `;
}

export async function findDeploymentById(id) {
  const rows = await sql`
    SELECT
      d.*,
      COALESCE(
        (
          SELECT json_agg(dd ORDER BY dd.created_at DESC)
          FROM deployment_domains dd
          WHERE dd.deployment_id = d.id
        ),
        '[]'::json
      ) AS domains
    FROM deployments d
    WHERE d.id = ${id}
  `;
  return rows[0] ?? null;
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
  await sql`
    INSERT INTO deployments (
      id, owner_id, name, repository, branch,
      blue_green_enabled, dockerized, status
    ) VALUES (${id}, ${ownerId}, ${name}, ${repository}, ${branch}, ${blueGreenEnabled}, ${dockerized}, 'pending')
  `;
  return findDeploymentById(id);
}

export async function updateDeployment(id, updates) {
  const rows = await sql`
    UPDATE deployments
    SET ${sql(updates)}, updated_at = NOW()
    WHERE id = ${id}
    RETURNING *
  `;
  return rows[0] ?? null;
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
  await sql`
    UPDATE deployment_versions
    SET status = 'inactive'
    WHERE deployment_id = ${deploymentId} AND slot = ${slot}
  `;
  await sql`
    INSERT INTO deployment_versions (
      id, deployment_id, slot, commit_sha, status, port,
      dockerized, artifact_path
    ) VALUES (${id}, ${deploymentId}, ${slot}, ${commitSha}, ${status}, ${port}, ${dockerized}, ${artifactPath})
  `;
  return id;
}

export async function getActiveVersion(deploymentId) {
  const rows = await sql`
    SELECT *
    FROM deployment_versions
    WHERE deployment_id = ${deploymentId} AND status = 'active'
    ORDER BY created_at DESC
    LIMIT 1
  `;
  return rows[0] ?? null;
}

export async function listVersions(deploymentId) {
  return sql`
    SELECT *
    FROM deployment_versions
    WHERE deployment_id = ${deploymentId}
    ORDER BY created_at DESC
    LIMIT 20
  `;
}

export async function recordDeploymentLog(deploymentId, level, message, meta = {}) {
  await sql`
    INSERT INTO deployment_logs (id, deployment_id, level, message, metadata)
    VALUES (${nanoid()}, ${deploymentId}, ${level}, ${message}, ${JSON.stringify(meta)}::jsonb)
  `;
}

export async function listDeploymentLogs(deploymentId, limit = 200) {
  return sql`
    SELECT *
    FROM deployment_logs
    WHERE deployment_id = ${deploymentId}
    ORDER BY created_at DESC
    LIMIT ${limit}
  `;
}

export async function deleteDeployment(id) {
  await sql`DELETE FROM deployments WHERE id = ${id}`;
}
