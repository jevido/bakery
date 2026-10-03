import type { DatabaseType } from './types'

/** The Database types in the order the API lists them, with their default versions. */
export const databaseTypes: { type: DatabaseType; label: string; defaultVersion: string }[] = [
  { type: 'postgresql', label: 'PostgreSQL', defaultVersion: '18-alpine' },
  { type: 'mysql', label: 'MySQL', defaultVersion: '8.4' },
  { type: 'mariadb', label: 'MariaDB', defaultVersion: '11' },
  { type: 'redis', label: 'Redis', defaultVersion: '8-alpine' },
  { type: 'valkey', label: 'Valkey', defaultVersion: '8-alpine' },
  { type: 'mongodb', label: 'MongoDB', defaultVersion: '8' },
]

export function databaseTypeLabel(type: DatabaseType): string {
  return databaseTypes.find((t) => t.type === type)?.label ?? type
}
