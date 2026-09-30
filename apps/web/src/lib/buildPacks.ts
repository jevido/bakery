import type { Application, BuildPack } from './types'

export const packLabel: Record<BuildPack, string> = {
  dockerfile: 'Dockerfile',
  nixpacks: 'Nixpacks',
  static: 'Static site',
  image: 'Image',
}

/** Where an application comes from, in one line. */
export function sourceLine(a: Application): string {
  return a.build_pack === 'image' ? a.image_reference : `${a.git_url} @ ${a.git_branch}`
}
