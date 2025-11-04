import { sql as globalSql, SQL } from 'bun';
import { getConfig } from './config.js';

let client;

function createClient() {
  const config = getConfig();
  if (config.databaseUrl) {
    return new SQL({
      url: config.databaseUrl,
      max: 10,
      ssl: config.environment === 'production' ? 'require' : undefined
    });
  }
  return globalSql;
}

function db() {
  if (!client) {
    client = createClient();
  }
  return client;
}

export function sql(strings, ...values) {
  return db()(strings, ...values);
}

sql.begin = (...args) => db().begin(...args);
sql.transaction = (...args) => db().transaction(...args);
sql.end = (...args) => db().end?.(...args);
sql.close = (...args) => db().close?.(...args);
sql.reserve = (...args) => db().reserve(...args);
sql.file = (...args) => db().file(...args);
sql.unsafe = (...args) => db().unsafe(...args);

export async function ensureConnection() {
  await sql`SELECT 1`;
}

export async function executeSimple(query) {
  if (typeof query === 'string') {
    return db().unsafe(query).simple();
  }
  return db()(query).simple();
}

export function resetDatabaseClient() {
  client = undefined;
}
