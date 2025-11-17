import { nanoid } from 'nanoid';
import { sql } from 'bun';

export async function listDeploymentsForUser(userId) {
	return sql`
    SELECT
      d.*,
      n.name AS node_name,
      n.status AS node_status,
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
    LEFT JOIN nodes n ON n.id = d.node_id
    WHERE d.owner_id = ${userId}
    ORDER BY d.updated_at DESC
  `;
}

export async function listDeploymentsByRepository(repository, branch) {
	return sql`
	  SELECT *
	  FROM deployments
	  WHERE repository = ${repository}
	    AND branch = ${branch}
	`;
}

export async function findDeploymentById(id) {
	const rows = await sql`
    SELECT
      d.*,
      n.name AS node_name,
      n.status AS node_status,
      COALESCE(
        (
          SELECT json_agg(dd ORDER BY dd.created_at DESC)
          FROM deployment_domains dd
          WHERE dd.deployment_id = d.id
        ),
        '[]'::json
      ) AS domains
    FROM deployments d
    LEFT JOIN nodes n ON n.id = d.node_id
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
		dockerized = false,
		nodeId = null,
		dockerfilePath = 'Dockerfile',
		buildContext = '.'
	} = payload;
	await sql`
    INSERT INTO deployments (
      id, owner_id, name, repository, branch,
      blue_green_enabled, dockerized, status, node_id,
      dockerfile_path, build_context
    ) VALUES (${id}, ${ownerId}, ${name}, ${repository}, ${branch}, ${blueGreenEnabled}, ${dockerized}, 'pending', ${nodeId}, ${dockerfilePath}, ${buildContext})
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
	const { deploymentId, slot, commitSha, status, port, dockerized, artifactPath } = payload;
	await sql`
    UPDATE deployment_versions
    SET status = 'inactive'
    WHERE deployment_id = ${deploymentId}
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
	try {
		await sql`
      INSERT INTO deployment_logs (id, deployment_id, level, message, metadata)
      VALUES (${nanoid()}, ${deploymentId}, ${level}, ${message}, ${JSON.stringify(meta)}::jsonb)
    `;
	} catch (error) {
		// Ignore foreign key errors when the deployment no longer exists.
		if (error?.code === '23503') {
			return;
		}
		throw error;
	}
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
	await sql`
    DELETE FROM tasks
    WHERE payload ->> 'deploymentId' = ${id}
  `;
	await sql`DELETE FROM deployments WHERE id = ${id}`;
}
