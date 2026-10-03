import type { Application, BuildPack } from './types'

export const packLabel: Record<BuildPack, string> = {
  dockerfile: 'Dockerfile',
  nixpacks: 'Nixpacks',
  static: 'Static site',
  dockerimage: 'Docker Image',
}

/** What the Source tab shows, in one line: the Git repository and branch, or the Docker image. */
export function sourceLine(a: Application): string {
  return a.build_pack === 'dockerimage' ? a.docker_image : `${a.git_url} @ ${a.git_branch}`
}
