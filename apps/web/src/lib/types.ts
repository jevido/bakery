// Shapes the API returns (services/api/contexts/*/http).

export type Application = {
  id: number
  project_id: number
  environment_id: number
  name: string
  slug: string
  build_pack: BuildPack
  /** Set for the dockerimage build pack, which has no Git repository. */
  docker_image: string
  /** What the static build pack serves. */
  publish_directory: string
  git_url: string
  git_branch: string
  dockerfile_path: string
  port: number
  /** 1–10 hostnames; the first is the primary one. */
  domains: string[]
  /** Empty for an https Git repository. */
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
  /** The Target server: where it is built and runs. Fixed once the application exists. */
  server_id: number
}

/** A persistent storage: the volume `name`, mounted at `mount_path` in every container. */
export type Storage = { name: string; mount_path: string }

/** null is unlimited. */
export type ResourceLimits = { memory_mb: number | null; cpus: number | null }

export type BuildPack = 'dockerfile' | 'nixpacks' | 'static' | 'dockerimage'

/** Times in seconds. */
export type HealthCheck = {
  enabled: boolean
  path: string
  interval: number
  timeout: number
  retries: number
  start_period: number
}

export type Redirect = 'both' | 'www' | 'non-www'

/** How the proxy treats an application's traffic; the password hash is never returned. */
export type RouteSettings = {
  redirect: Redirect
  response_headers: { name: string; value: string }[]
  basic_auth: { enabled: boolean; username: string; password_set: boolean }
}

export type Webhook = {
  path: string
  secret: string
  auto_deploy: boolean
  /** Pull requests start Previews. */
  previews: boolean
  /** Whether a Git host token is saved; the token itself never comes back. */
  has_git_host_token: boolean
}

export type Preview = {
  number: number
  title: string
  branch: string
  /** The pull request's page on the git host. */
  url: string
  provider: string
  state: 'open' | 'closed'
  /** pr-<number>.<primary domain> */
  domain: string
  public_url: string
  commented: boolean
  latest_deployment: Deployment | null
  created_at: string
  closed_at: string | null
}

export type KnownHost = { id: number; host: string; fingerprints: string[]; created_at: string }

export type Environment = {
  id: number
  project_id: number
  project_name?: string
  name: string
  description: string
  applications: Application[]
}

export type Project = {
  id: number
  name: string
  description: string
  environments?: Environment[]
}

/** build: handed to the build as a build arg; runtime: set in the container. */
export type EnvironmentVariable = { name: string; value: string; build: boolean; runtime: boolean }

/** A shared variable as seen from an application; overridden when a narrower level sets the same name. */
export type InheritedVariable = EnvironmentVariable & { from: 'project' | 'environment'; overridden: boolean }

export type ApplicationInput = Pick<
  Application,
  'name' | 'build_pack' | 'docker_image' | 'publish_directory' | 'git_url' | 'git_branch' | 'dockerfile_path' | 'port' | 'domains'
> & {
  /** Omitted keeps the current one. */
  health_check?: HealthCheck
  /** Omitted keeps the current ones; an empty username removes them; an empty password keeps the stored one. */
  registry_credentials?: { username: string; password: string }
  /** Omitted keeps the current ones. */
  storages?: Storage[]
  /** Omitted keeps the current ones. */
  resource_limits?: ResourceLimits
  /** Only read when the application is created; omitted is the local server. */
  server_id?: number
}

export type DeploymentStatus = 'queued' | 'cloning' | 'building' | 'starting' | 'finished' | 'failed' | 'cancelled'

export type Deployment = {
  id: number
  application_id: number
  /** The pull request number of a Preview Deployment, 0 for the application itself. */
  preview: number
  /** The server it ran on. */
  server_id: number
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

export type DatabaseType = 'postgresql' | 'mysql' | 'mariadb' | 'redis' | 'valkey' | 'mongodb'

/** Read from Podman: starting until it answers its Database type's readiness probe. */
export type DatabaseStatus = 'starting' | 'running' | 'stopped' | 'exited' | 'missing'

export type Database = {
  id: number
  environment_id: number
  project_id: number
  name: string
  slug: string
  type: DatabaseType
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
  /** Set when the credentials and URLs were left out for a viewer. */
  secrets_hidden?: boolean
  internal_url?: string
  public_url?: string | null
  /** false for Redis and Valkey. */
  backups_supported: boolean
  scheduled_backup: ScheduledBackup
  /** When the schedule fires next (UTC); null when it is off. */
  next_backup_at: string | null
  restoring: boolean
  last_restore: { backup_execution_id: number; started_at: string; finished_at: string; error?: string } | null
}

export type ScheduledBackup = {
  enabled: boolean
  /** Five-field cron expression, in UTC. */
  cron: string
  retention: number
  /** null: local disk only. */
  s3_storage_id: number | null
}

export type ExecutionStatus = 'running' | 'succeeded' | 'failed'

export type BackupExecution = {
  id: number
  database_id: number
  status: ExecutionStatus
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
  type?: DatabaseType
  version: string
  public_port: number | null
  resource_limits: ResourceLimits
}

/** An S3-compatible bucket Backup executions are uploaded to; the secret is write-only. */
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

/** Summed up from the Components: deploying while an action runs. */
export type ServiceStatus = 'running' | 'stopped' | 'deploying' | 'degraded' | 'failed'

/** One entry under the compose file's services:, run as one container. */
export type Component = {
  name: string
  image: string
  /** Public components have a port and domains; the proxy serves them. */
  public: boolean
  port: number | null
  domains: string[]
  url?: string
  status: DatabaseStatus
  detail?: string
}

export type ServiceVariable = {
  name: string
  value: string
  /** Generated by The Bakery ("password", "user", "base64", …); "" when the Owner sets it. */
  /** Set when the value was left out for a viewer. */
  hidden?: boolean
  magic: string
  /** The compose file's ${NAME:-default}; null without one. */
  default: string | null
}

export type Service = {
  id: number
  environment_id: number
  project_id: number
  name: string
  slug: string
  template?: string
  status: ServiceStatus
  desired_state: 'running' | 'stopped'
  busy: boolean
  last_error?: string
  components: Component[]
  /** Only on a single service, not in lists. */
  compose?: string
  variables?: ServiceVariable[]
}

export type ServiceTemplate = {
  key: string
  name: string
  description: string
  docs_url: string
  tags: string[]
}

/** Either a template key or a compose file. */
export type ServiceInput = { name: string; template?: string; compose?: string }

export type ServerStatus = 'unvalidated' | 'reachable' | 'unreachable'

export type ServerCheck = {
  name: 'ssh' | 'socket' | 'podman' | 'linger' | 'ports'
  ok: boolean
  required: boolean
  detail: string
}

export type Server = {
  id: number
  name: string
  kind: 'local' | 'remote'
  host: string
  port: number
  user: string
  status: ServerStatus
  validation: { checks: ServerCheck[]; checked_at: string | null }
  last_cleanup: { at: string | null; reclaimed_bytes: number }
  created_at: string
  /** Only on a single Server. */
  public_key?: string
  host_key_fingerprint?: string
}

export type ServerMetrics = {
  cpus: number
  cpu_percent: number
  memory_used_bytes: number
  memory_total_bytes: number
  disk_used_bytes: number
  disk_total_bytes: number
  images_bytes: number
  containers_bytes: number
  volumes_bytes: number
  read_at: string
}

export type ContainerMetrics = {
  name: string
  owner: 'application' | 'database' | 'service' | 'proxy' | ''
  owner_id: string
  cpu_percent: number
  memory_used_bytes: number
  memory_limit_bytes: number
}

export type Metrics = { server: ServerMetrics; containers: ContainerMetrics[] }

export type { Member, Role } from './session.svelte'

export type Invitation = {
  id: number
  email: string
  role: 'admin' | 'member' | 'viewer'
  created_at: string
  expires_at: string
}

export type ChannelKind = 'email' | 'discord' | 'slack' | 'telegram' | 'ntfy' | 'webhook'

export type EventKind = {
  kind: string
  label: string
  /** Whether a new channel is subscribed to it. */
  default: boolean
}

/** A Notification channel as shown: secrets only as whether they are set. */
export type NotificationChannel = {
  id: number
  name: string
  kind: ChannelKind
  settings: {
    host?: string
    port?: number
    security?: 'none' | 'starttls' | 'tls'
    username?: string
    has_password: boolean
    from?: string
    to?: string[]
    /** ntfy's server; the other kinds' URLs are secret. */
    url?: string
    /** Of a secret URL, only its host. */
    url_host?: string
    chat_id?: string
    has_bot_token: boolean
    topic?: string
    has_token: boolean
    has_secret: boolean
  }
  event_kinds: string[]
  created_at: string
}

export type Delivery = {
  id: number
  event_kind: string
  title: string
  status: 'pending' | 'sent' | 'failed'
  attempts: number
  last_error: string
  created_at: string
  sent_at: string | null
}
