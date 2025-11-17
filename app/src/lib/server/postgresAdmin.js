import { randomBytes } from 'node:crypto';
import { sql } from 'bun';
import { getConfig } from './config.js';
import { log } from './logger.js';
import { runSshCommand } from './sshClient.js';

export function parseDatabaseUrl(url) {
	const parsed = new URL(url);
	return {
		host: parsed.hostname,
		port: parsed.port || '5432',
		user: decodeURIComponent(parsed.username),
		password: decodeURIComponent(parsed.password || ''),
		database: parsed.pathname.replace('/', '') || 'postgres'
	};
}

function formatStatement(strings) {
	return strings.reduce((acc, part, index) => acc + part + (index < strings.length - 1 ? `$${index + 1}` : ''), '');
}

async function runPsql(strings, ...values) {
	try {
		return await sql(strings, ...values);
	} catch (error) {
		await log('error', 'psql command failed', {
			sql: formatStatement(strings),
			error: error instanceof Error ? error.message : String(error)
		});
		throw error;
	}
}

async function runUnsafe(statement) {
	try {
		return await sql.unsafe(statement);
	} catch (error) {
		await log('error', 'psql command failed', {
			sql: statement,
			error: error instanceof Error ? error.message : String(error)
		});
		throw error;
	}
}

function randomString(length = 24) {
	return randomBytes(length).toString('hex').slice(0, length);
}

function quoteIdentifier(value) {
	if (!/^[a-zA-Z0-9_]+$/.test(value)) {
		throw new Error('Invalid identifier');
	}
	return `"${value.replace(/"/g, '""')}"`;
}

function escapeLiteral(value) {
	return `'${value.replace(/'/g, "''")}'`;
}

export async function provisionDatabase(deploymentId) {
	const sanitized = deploymentId.replace(/[^a-zA-Z0-9]/g, '');
	const dbName = `bakery_${sanitized.slice(-8)}_${randomString(6)}`;
	const dbUser = `bakery_u_${randomString(8)}`;
	const dbPassword = randomString(32);

	const createRoleStatement = `CREATE ROLE ${quoteIdentifier(dbUser)} WITH LOGIN PASSWORD ${escapeLiteral(dbPassword)};`;
	await runUnsafe(createRoleStatement);

	const createDbStatement = `CREATE DATABASE ${quoteIdentifier(dbName)} OWNER ${quoteIdentifier(dbUser)};`;
	await runUnsafe(createDbStatement);

	const config = getConfig();
	const parsed = parseDatabaseUrl(config.databaseUrl);
	const connectionUrl = `postgres://${dbUser}:${encodeURIComponent(dbPassword)}@${parsed.host}:${parsed.port}/${dbName}`;

	return {
		name: dbName,
		user: dbUser,
		password: dbPassword,
		connectionUrl
	};
}

export async function dropDatabase(dbName) {
	const dropStatement = `DROP DATABASE IF EXISTS ${quoteIdentifier(dbName)};`;
	await runUnsafe(dropStatement);
}

function sanitizeIdentifier(value, fallback) {
	const normalized = value.replace(/[^a-zA-Z0-9_]/g, '_');
	return normalized || fallback;
}

function escapeSqlIdentifier(value) {
	return value.replace(/"/g, '""');
}

function escapeSqlLiteral(value) {
	return value.replace(/'/g, "''");
}

async function runRemoteScript(node, script) {
	if (!node?.ssh_private_key || !node?.ssh_host) {
		throw new Error('Node is missing SSH credentials or host metadata');
	}
	return runSshCommand(
		{
			host: node.ssh_host,
			port: node.ssh_port || 22,
			user: node.ssh_user || 'bakery-agent',
			privateKey: node.ssh_private_key
		},
		script
	);
}

export async function provisionDatabaseOnNode(node, deploymentId) {
	const sanitized = sanitizeIdentifier(deploymentId, randomString(8));
	const dbName = `bakery_${sanitized.slice(-8)}_${randomString(6)}`;
	const dbUser = `bakery_u_${randomString(8)}`;
	const dbPassword = randomString(32);
	const sqlBlock = String.raw`
DO $$BEGIN
   IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = '${escapeSqlLiteral(dbUser)}') THEN
      EXECUTE 'CREATE ROLE "${escapeSqlIdentifier(dbUser)}" LOGIN PASSWORD ''${escapeSqlLiteral(dbPassword)}''';
   ELSE
      EXECUTE 'ALTER ROLE "${escapeSqlIdentifier(dbUser)}" WITH PASSWORD ''${escapeSqlLiteral(dbPassword)}''';
   END IF;
END$$;
CREATE DATABASE "${escapeSqlIdentifier(dbName)}" OWNER "${escapeSqlIdentifier(dbUser)}";
GRANT ALL PRIVILEGES ON DATABASE "${escapeSqlIdentifier(dbName)}" TO "${escapeSqlIdentifier(dbUser)}";
`;

	const command = String.raw`
set -euo pipefail

if ! command -v psql >/dev/null; then
  if command -v apt-get >/dev/null; then
    sudo apt-get update -y
    sudo apt-get install -y postgresql postgresql-contrib
  else
    echo "PostgreSQL is not installed and apt-get is unavailable on this host." >&2
    exit 1
  fi
fi

sudo systemctl enable --now postgresql

PG_CONF=$(sudo -u postgres psql -tAc "SHOW config_file;")
PG_HBA=$(sudo -u postgres psql -tAc "SHOW hba_file;")

if [ -n "$PG_CONF" ] && ! sudo grep -q "listen_addresses = '*'" "$PG_CONF"; then
  sudo sed -i "s/^#\?listen_addresses\s*=.*/listen_addresses = '*'/" "$PG_CONF" || echo "listen_addresses = '*'" | sudo tee -a "$PG_CONF" >/dev/null
fi

if [ -n "$PG_HBA" ] && ! sudo grep -q "host all all 127.0.0.1/32 md5" "$PG_HBA"; then
  echo "host all all 127.0.0.1/32 md5" | sudo tee -a "$PG_HBA" >/dev/null
fi

if [ -n "$PG_HBA" ] && ! sudo grep -q "host all all 172.16.0.0/12 md5" "$PG_HBA"; then
  echo "host all all 172.16.0.0/12 md5" | sudo tee -a "$PG_HBA" >/dev/null
fi

sudo systemctl restart postgresql

sudo -u postgres psql <<'SQL'
${sqlBlock}
SQL
`;

	await runRemoteScript(node, command);

	const connectionHost = node.ssh_host;
	const connectionUrl = `postgres://${dbUser}:${encodeURIComponent(dbPassword)}@${connectionHost}:5432/${dbName}`;
	return {
		name: dbName,
		user: dbUser,
		password: dbPassword,
		connectionUrl
	};
}

export async function dropDatabaseOnNode(node, databaseName, roleName) {
	if (!databaseName) {
		throw new Error('Database name is required to drop a remote database');
	}
	const dbIdentifier = escapeSqlIdentifier(databaseName);
	const roleIdentifier = roleName ? escapeSqlIdentifier(roleName) : null;
	const dropSql = String.raw`
SELECT pg_terminate_backend(pid)
FROM pg_stat_activity
WHERE datname = '${escapeSqlLiteral(databaseName)}';
DROP DATABASE IF EXISTS "${dbIdentifier}";
${roleIdentifier ? `DROP ROLE IF EXISTS "${roleIdentifier}";` : ''}
`;

	const command = String.raw`
set -euo pipefail
if ! command -v psql >/dev/null; then
  echo "PostgreSQL is not installed on this host." >&2
  exit 1
fi
sudo systemctl enable --now postgresql >/dev/null 2>&1 || true
sudo -u postgres psql <<'SQL'
${dropSql}
SQL
`;

	await runRemoteScript(node, command);
}
