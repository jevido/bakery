// Shapes the API returns (services/api/contexts/*/http).

export type Application = {
  id: number
  project_id: number
  environment_id: number
  name: string
  slug: string
  git_url: string
  git_branch: string
  dockerfile_path: string
  port: number
  domain: string
  /** Empty for an https Source. */
  deploy_key_public: string
  public_url: string
  health_check: HealthCheck
}

/** Times in seconds. */
export type HealthCheck = {
  enabled: boolean
  path: string
  interval: number
  timeout: number
  retries: number
  start_period: number
}

export type Webhook = { path: string; secret: string; auto_deploy: boolean }

export type KnownHost = { id: number; host: string; fingerprints: string[]; created_at: string }

export type Environment = { id: number; name: string; applications: Application[] }

export type Project = {
  id: number
  name: string
  description: string
  environments?: Environment[]
}

/** build: handed to the build as a build arg; runtime: set in the container. */
export type EnvVar = { name: string; value: string; build: boolean; runtime: boolean }

/** A shared variable as seen from an application; overridden when a narrower level sets the same name. */
export type InheritedVariable = EnvVar & { from: 'project' | 'environment'; overridden: boolean }

export type ApplicationInput = Pick<Application, 'name' | 'git_url' | 'git_branch' | 'dockerfile_path' | 'port' | 'domain'> & {
  /** Omitted keeps the current one. */
  health_check?: HealthCheck
}

export type DeploymentStatus = 'queued' | 'cloning' | 'building' | 'starting' | 'finished' | 'failed' | 'cancelled'

export type Deployment = {
  id: number
  application_id: number
  status: DeploymentStatus
  active: boolean
  trigger: 'manual' | 'webhook' | 'rollback'
  branch: string
  commit_sha: string
  commit_message: string
  commit_author: string
  image: string
  container: string
  /** The deployment whose image a rollback starts again. */
  rollback_of: number | null
  error: string
  created_at: string
  started_at: string | null
  finished_at: string | null
}

export type LogLine = { stream: 'info' | 'out' | 'err'; line: string }
