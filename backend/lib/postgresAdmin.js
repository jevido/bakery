import { randomBytes } from 'node:crypto';
import { spawn } from 'bun';
import { getConfig } from './config.js';
import { log } from './logger.js';

function parseDatabaseUrl(url) {
  const parsed = new URL(url);
  return {
    host: parsed.hostname,
    port: parsed.port || '5432',
    user: decodeURIComponent(parsed.username),
    password: decodeURIComponent(parsed.password || ''),
    database: parsed.pathname.replace('/', '') || 'postgres'
  };
}

async function runPsql(sql) {
  const config = getConfig();
  const parsed = parseDatabaseUrl(config.databaseUrl);
  const process = spawn(
    [
      'psql',
      '-h',
      parsed.host,
      '-p',
      parsed.port,
      '-U',
      parsed.user,
      '-d',
      'postgres',
      '-c',
      sql
    ],
    {
      env: {
        ...process.env,
        PGPASSWORD: parsed.password
      },
      stdout: 'pipe',
      stderr: 'pipe'
    }
  );
  const stdout = await new Response(process.stdout).text();
  const stderr = await new Response(process.stderr).text();
  if (process.exitCode !== 0) {
    await log('error', 'psql command failed', { sql, stderr });
    throw new Error(stderr);
  }
  return stdout.trim();
}

function randomString(length = 24) {
  return randomBytes(length).toString('hex').slice(0, length);
}

export async function provisionDatabase(deploymentId) {
  const sanitized = deploymentId.replace(/[^a-zA-Z0-9]/g, '');
  const dbName = `bakery_${sanitized.slice(-8)}_${randomString(6)}`;
  const dbUser = `bakery_u_${randomString(8)}`;
  const dbPassword = randomString(32);

  await runPsql(`CREATE ROLE "${dbUser}" WITH LOGIN PASSWORD '${dbPassword}';`);
  await runPsql(`CREATE DATABASE "${dbName}" OWNER "${dbUser}";`);

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
  await runPsql(`DROP DATABASE IF EXISTS "${dbName}";`);
}
