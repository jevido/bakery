// Shapes the API returns (services/api/contexts/*/http).

export type Application = {
  id: number
  project_id: number
  environment_id: number
  name: string
  slug: string
  build_pack: BuildPack
  /** Set for the image build pack, which has no git source. */
  image_reference: string
  /** What the static build pack serves. */
  publish_directory: string
  git_url: string
  git_branch: string
  dockerfile_path: string
  port: number
  /** 1–10 hostnames; the first is the primary one. */
  domains: string[]
  /** Empty for an https Source. */
  deploy_key_public: string
  /** On the primary Domain. */
  public_url: string
  public_urls: string[]
  health_check: HealthCheck
  registry_username: string
  /** The password itself is never returned. */
  has_registry_password: boolean
  storages: Storage[]
  resource_limits: ResourceLimits
}

/** A persistent storage: the volume `name`, mounted at `mount_path` in every container. */
export type Storage = { name: string; mount_path: string }

/** null is unlimited. */
export type ResourceLimits = { memory_mb: number | null; cpus: number | null }

export type BuildPack = 'dockerfile' | 'nixpacks' | 'static' | 'image'

/** Times in seconds. */
export type HealthCheck = {
  enabled: boolean
  path: string
  interval: number
  timeout: number
  retries: number
  start_period: number
}

export type WwwRedirect = 'off' | 'to_apex' | 'to_www'

/** How the proxy treats an application's traffic; the password hash is never returned. */
export type RouteSettings = {
  www_redirect: WwwRedirect
  response_headers: { name: string; value: string }[]
  basic_auth: { enabled: boolean; username: string; password_set: boolean }
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

export type ApplicationInput = Pick<
  Application,
  'name' | 'build_pack' | 'image_reference' | 'publish_directory' | 'git_url' | 'git_branch' | 'dockerfile_path' | 'port' | 'domains'
> & {
  /** Omitted keeps the current one. */
  health_check?: HealthCheck
  /** Omitted keeps the current ones; an empty username removes them; an empty password keeps the stored one. */
  registry_credentials?: { username: string; password: string }
  /** Omitted keeps the current ones. */
  storages?: Storage[]
  /** Omitted keeps the current ones. */
  resource_limits?: ResourceLimits
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
  /** The reference with digest an image deployment pulled. */
  source_image: string
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

export type Engine = 'postgresql' | 'mysql' | 'mariadb' | 'redis' | 'valkey' | 'mongodb'

/** Read from Podman: starting until it answers its engine's readiness probe. */
export type DatabaseStatus = 'starting' | 'running' | 'stopped' | 'exited' | 'missing'

export type Database = {
  id: number
  environment_id: number
  project_id: number
  name: string
  slug: string
  engine: Engine
  version: string
  status: DatabaseStatus
  desired_state: 'running' | 'stopped'
  /** Why the last start failed, or how the container exited. */
  error?: string
  /** null: not published. */
  public_port: number | null
  resource_limits: ResourceLimits
  /** Only on a single database, not in lists. */
  credentials?: { username: string; password: string; root_password?: string; database_name: string }
  internal_url?: string
  public_url?: string | null
  /** false for Redis and Valkey. */
  backups_supported: boolean
  backup_schedule: BackupSchedule
  /** When the schedule fires next (UTC); null when it is off. */
  next_backup_at: string | null
  restoring: boolean
  last_restore: { backup_id: number; started_at: string; finished_at: string; error?: string } | null
}

export type BackupSchedule = {
  enabled: boolean
  /** Five-field cron expression, in UTC. */
  cron: string
  retention: number
  /** null: local disk only. */
  s3_storage_id: number | null
}

export type BackupStatus = 'running' | 'succeeded' | 'failed'

export type Backup = {
  id: number
  database_id: number
  status: BackupStatus
  trigger: 'manual' | 'scheduled'
  file_name: string
  size_bytes: number
  local: boolean
  s3: boolean
  error?: string
  started_at: string
  finished_at: string | null
}

export type DatabaseInput = {
  name: string
  engine?: Engine
  version: string
  public_port: number | null
  resource_limits: ResourceLimits
}

/** An S3-compatible bucket Backups are uploaded to; the secret is write-only. */
export type S3Storage = {
  id: number
  name: string
  endpoint: string
  region: string
  bucket: string
  prefix: string
  access_key: string
  has_secret_key: boolean
}

export type S3StorageInput = {
  name: string
  endpoint: string
  region: string
  bucket: string
  prefix: string
  access_key: string
  /** Empty on an update keeps the stored one. */
  secret_key: string
}
