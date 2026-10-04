import type { Application, ApplicationInput } from "../../lib/types";

/**
 * The fields PATCH /applications/{id} always takes, as the Application has
 * them now; the optional ones left out keep their current values.
 */
export function applicationInput(a: Application): ApplicationInput {
  return {
    name: a.name,
    build_pack: a.build_pack,
    docker_image: a.docker_image,
    publish_directory: a.publish_directory,
    git_url: a.git_url,
    git_branch: a.git_branch,
    dockerfile_path: a.dockerfile_path,
    port: a.port,
    domains: a.domains,
  };
}
