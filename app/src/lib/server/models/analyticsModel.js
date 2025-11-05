import { sql } from 'bun';

export async function recordTrafficSnapshot({
  deploymentId,
  visits,
  bandwidth,
  timestamp
}) {
  await sql`
    INSERT INTO analytics_snapshots (
      deployment_id, visits, bandwidth, captured_at
    ) VALUES (${deploymentId}, ${visits}, ${bandwidth}, ${timestamp})
  `;
}

export async function recordDiskSnapshot({
  deploymentId,
  usedBytes,
  timestamp
}) {
  await sql`
    INSERT INTO disk_snapshots (
      deployment_id, used_bytes, captured_at
    ) VALUES (${deploymentId}, ${usedBytes}, ${timestamp})
  `;
}

export async function recordDatabaseSnapshot({
  deploymentId,
  databaseSize,
  timestamp
}) {
  await sql`
    INSERT INTO database_snapshots (
      deployment_id, size_bytes, captured_at
    ) VALUES (${deploymentId}, ${databaseSize}, ${timestamp})
  `;
}

export async function getRecentAnalytics(deploymentId, days = 14) {
  return sql`
    SELECT
      'traffic' AS type,
      visits AS value_a,
      bandwidth AS value_b,
      captured_at
    FROM analytics_snapshots
    WHERE deployment_id = ${deploymentId}
      AND captured_at > NOW() - (${days} || ' days')::interval
    UNION ALL
    SELECT
      'disk' AS type,
      used_bytes AS value_a,
      NULL AS value_b,
      captured_at
    FROM disk_snapshots
    WHERE deployment_id = ${deploymentId}
      AND captured_at > NOW() - (${days} || ' days')::interval
    UNION ALL
    SELECT
      'database' AS type,
      size_bytes AS value_a,
      NULL AS value_b,
      captured_at
    FROM database_snapshots
    WHERE deployment_id = ${deploymentId}
      AND captured_at > NOW() - (${days} || ' days')::interval
    ORDER BY captured_at DESC
  `;
}

export async function getDiskUsageTrend(hours = 24) {
  return sql`
    SELECT
      date_trunc('hour', captured_at) AS bucket,
      SUM(used_bytes)::bigint AS used_bytes
    FROM disk_snapshots
    WHERE captured_at > NOW() - (${hours} || ' hours')::interval
    GROUP BY bucket
    ORDER BY bucket ASC
  `;
}
