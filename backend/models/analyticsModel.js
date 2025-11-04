import { query } from '../lib/db.js';

export async function recordTrafficSnapshot({
  deploymentId,
  visits,
  bandwidth,
  timestamp
}) {
  await query(
    `
      INSERT INTO analytics_snapshots (
        deployment_id, visits, bandwidth, captured_at
      ) VALUES ($1, $2, $3, $4)
    `,
    [deploymentId, visits, bandwidth, timestamp]
  );
}

export async function recordDiskSnapshot({
  deploymentId,
  usedBytes,
  timestamp
}) {
  await query(
    `
      INSERT INTO disk_snapshots (
        deployment_id, used_bytes, captured_at
      ) VALUES ($1, $2, $3)
    `,
    [deploymentId, usedBytes, timestamp]
  );
}

export async function recordDatabaseSnapshot({
  deploymentId,
  databaseSize,
  timestamp
}) {
  await query(
    `
      INSERT INTO database_snapshots (
        deployment_id, size_bytes, captured_at
      ) VALUES ($1, $2, $3)
    `,
    [deploymentId, databaseSize, timestamp]
  );
}

export async function getRecentAnalytics(deploymentId, days = 14) {
  return query(
    `
      SELECT
        'traffic' AS type,
        visits AS value_a,
        bandwidth AS value_b,
        captured_at
      FROM analytics_snapshots
      WHERE deployment_id = $1
        AND captured_at > NOW() - ($2 || ' days')::interval
      UNION ALL
      SELECT
        'disk' AS type,
        used_bytes AS value_a,
        NULL AS value_b,
        captured_at
      FROM disk_snapshots
      WHERE deployment_id = $1
        AND captured_at > NOW() - ($2 || ' days')::interval
      UNION ALL
      SELECT
        'database' AS type,
        size_bytes AS value_a,
        NULL AS value_b,
        captured_at
      FROM database_snapshots
      WHERE deployment_id = $1
        AND captured_at > NOW() - ($2 || ' days')::interval
      ORDER BY captured_at DESC
    `,
    [deploymentId, days]
  );
}
