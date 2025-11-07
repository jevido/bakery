import { randomBytes } from 'node:crypto';
import { sql } from 'bun';
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

function randomString(length = 24) {
	return randomBytes(length).toString('hex').slice(0, length);
}

export async function provisionDatabase(deploymentId) {
	const sanitized = deploymentId.replace(/[^a-zA-Z0-9]/g, '');
	const dbName = `bakery_${sanitized.slice(-8)}_${randomString(6)}`;
	const dbUser = `bakery_u_${randomString(8)}`;
	const dbPassword = randomString(32);

	await runPsql`CREATE ROLE "${dbUser}" WITH LOGIN PASSWORD ${dbPassword};`;
	await runPsql`CREATE DATABASE "${dbName}" OWNER "${dbUser}";`;

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
	await runPsql`DROP DATABASE IF EXISTS "${dbName}";`;
}
