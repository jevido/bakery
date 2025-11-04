import { readdir, readFile } from 'node:fs/promises';
import { join } from 'node:path';
import { sql } from 'bun';
import { log } from './logger.js';

async function ensureTable() {
  await sql`
    CREATE TABLE IF NOT EXISTS schema_migrations (
      id SERIAL PRIMARY KEY,
      filename TEXT UNIQUE NOT NULL,
      executed_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
    )
  `;
}

async function appliedMigrations() {
  const rows = await sql`SELECT filename FROM schema_migrations`;
  return new Set(rows.map((row) => row.filename));
}

async function applyMigration(file) {
  const sqlText = await readFile(file, 'utf8');
  await log('info', 'Applying migration', { file });
  await sql.unsafe(sqlText);
  await sql`
    INSERT INTO schema_migrations (filename)
    VALUES (${file.split('/').pop()})
  `;
}

export async function migrate() {
  await ensureTable();
  const migrationsDir = join(process.cwd(), 'backend', 'migrations');
  const files = (await readdir(migrationsDir))
    .filter((file) => file.endsWith('.sql'))
    .sort();
  const applied = await appliedMigrations();
  for (const file of files) {
    if (applied.has(file)) {
      continue;
    }
    await applyMigration(join(migrationsDir, file));
  }
}

if (import.meta.main) {
  migrate()
    .then(() => {
      console.log('Migrations applied');
      process.exit(0);
    })
    .catch((error) => {
      console.error('Migration failed', error);
      process.exit(1);
    });
}
