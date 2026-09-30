import type { Engine } from './types'

/** The engines in the order the API lists them, with their default versions. */
export const engines: { engine: Engine; label: string; defaultVersion: string }[] = [
  { engine: 'postgresql', label: 'PostgreSQL', defaultVersion: '18-alpine' },
  { engine: 'mysql', label: 'MySQL', defaultVersion: '8.4' },
  { engine: 'mariadb', label: 'MariaDB', defaultVersion: '11' },
  { engine: 'redis', label: 'Redis', defaultVersion: '8-alpine' },
  { engine: 'valkey', label: 'Valkey', defaultVersion: '8-alpine' },
  { engine: 'mongodb', label: 'MongoDB', defaultVersion: '8' },
]

export function engineLabel(engine: Engine): string {
  return engines.find((e) => e.engine === engine)?.label ?? engine
}
