import { Database } from 'bun:postgres';
import { getConfig } from './config.js';

let database;

export function getDatabase() {
  if (!database) {
    const config = getConfig();
    database = new Database(config.databaseUrl, {
      ssl: config.environment === 'production' ? 'require' : undefined,
      max: 10
    });
  }
  return database;
}

export async function query(text, params = []) {
  const db = getDatabase();
  const result = await db.query(text, params);
  return result.rows || [];
}

export async function single(text, params = []) {
  const rows = await query(text, params);
  return rows[0] || null;
}

export async function transaction(callback) {
  const db = getDatabase();
  const client = await db.connect();
  try {
    await client.query('BEGIN');
    const result = await callback({
      query: (text, params = []) => client.query(text, params),
      single: (text, params = []) =>
        client.query(text, params).then((res) => res.rows?.[0] || null)
    });
    await client.query('COMMIT');
    return result;
  } catch (error) {
    await client.query('ROLLBACK');
    throw error;
  } finally {
    client.release();
  }
}

export async function ensureConnection() {
  await query('SELECT 1');
}
