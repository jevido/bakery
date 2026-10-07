// Shapes the API returns (services/api/contexts/*/http).

export type Application = {
  id: number
  project_id: number
  environment_id: number
  name: string
  /** Empty for none. */
  description: string
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
  /** `<slug>.<domain suffix>`, the Domain it gets when it has none. */
  generated_domain: string
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
  /** The signed-in Member's Permissions in the Project, overrides applied; only on GET /api/projects/{id}. */
  permissions?: import('./session.svelte').Permission[]
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
  description?: string
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
  trigger: 'manual' | 'webhook' | 'rollback' | 'restart'
  branch: string
  commit_sha: string
  commit_message: string
  commit_author: string
  /** The reference with digest an image deployment pulled. */
  source_image: string
  image: string
  container: string
  /** The deployment whose image a rollback or a restart starts again. */
  rollback_of: number | null
  /** Built without the build cache (Deploy without cache). */
  force_rebuild: boolean
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
  description: string
  slug: string
  type: DatabaseType
  version: string
  /** Coolify's Image field: the Docker Hub repository and the version, "postgres:18-alpine". */
  image: string
  status: DatabaseStatus
  desired_state: 'running' | 'stopped'
  /** Why the last start failed, or how the container exited. */
  error?: string
  /** null: not published. */
  public_port: number | null
  resource_limits: ResourceLimits
  /** The Container's name, as Runtime Logs show it. */
  container: string
  /** The data volume and where the Container mounts it. */
  volume: Storage
  /** Only on a single database, not in lists. */
  credentials?: { username: string; password: string; root_password?: string; database_name: string }
  /** Set when the credentials and URLs were left out for a viewer. */
  secrets_hidden?: boolean
  internal_url?: string
  public_url?: string | null
  /** false for Redis and Valkey. */
  backups_supported: boolean
  restoring: boolean
  last_restore: { backup_execution_id: number; started_at: string; finished_at: string; error?: string } | null
}

export type ScheduledBackup = {
  id: number
  database_id: number
  enabled: boolean
  /** Coolify's Frequency: a five-field cron expression in UTC, or a shortcut such as daily. */
  cron: string
  retention: number
  /** null: local disk only. */
  s3_storage_id: number | null
  /** When it fires next (UTC); absent when it is off. */
  next_backup_at?: string
}

/** A Scheduled backup as a Member sets it. */
export type ScheduledBackupInput = Omit<ScheduledBackup, 'id' | 'database_id' | 'next_backup_at'>

export type ExecutionStatus = 'running' | 'succeeded' | 'failed'

export type BackupExecution = {
  id: number
  database_id: number
  scheduled_backup_id: number
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
  description?: string
  type?: DatabaseType
  /** Either the version (the image tag) or the whole image; image wins. */
  version?: string
  image?: string
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
  /** The Domain Generate domain fills in; public only. */
  generated_domain?: string
  status: DatabaseStatus
  detail?: string
  /** The name of its Container, as its Runtime Logs card shows it. */
  container: string
  /** Its volume mounts from the Compose file, named as the Podman volume. */
  volumes: ComponentVolume[]
}

export type ComponentVolume = {
  name: string
  path: string
  read_only: boolean
}

export type ServiceVariable = {
  name: string
  value: string
  /** Set when the value was left out for a viewer. */
  hidden?: boolean
  /** Generated by The Bakery ("password", "user", "base64", …); "" when a Member sets it. */
  magic: string
  /** The compose file's ${NAME:-default}; null without one. */
  default: string | null
  /** The Components whose fields use it, in Compose file order. */
  components: string[]
}

export type Service = {
  id: number
  environment_id: number
  project_id: number
  name: string
  description: string
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
  /** Coolify's template category, e.g. monitoring. */
  category: string
  /** A file under the dashboard's svgs/. */
  logo: string
  tags: string[]
}

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
  description: string
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

/** Read live from the Server's Podman; `up_since` is null when unknown. */
export type ServerDetails = {
  os: string
  arch: string
  kernel: string
  cpus: number
  memory_bytes: number
  podman_version: string
  up_since: string | null
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

export type { CurrentGuild, GuildPlace, Member, Offer, Person, Role, RoleRef } from './session.svelte'

/** The Current guild as its General and Danger Zone pages show it. */
export type GuildDetails = {
  id: number
  name: string
  description: string
  /** Numbers its Issues, e.g. DEF in DEF-12. */
  issue_prefix: string
  /** What still keeps it from being deleted, e.g. "projects". */
  blocking: string[]
  /** Its Guild Master; left out after an edit. */
  guild_master?: import('./session.svelte').Person
  /** Its open Transfer offer, null without one. */
  offer: import('./session.svelte').Offer | null
}

/** A Role of the Current guild as GET /api/roles lists it. */
export type GuildRole = {
  id: number
  name: string
  color: string
  /** 0 for @everyone, contiguous from 1 above it. */
  position: number
  permissions: import('./session.svelte').Permission[]
  /** Whether it is @everyone, the Base role every Member holds. */
  base: boolean
  /** How many Members hold it. */
  members: number
}

/** A Permission from the fixed list GET /api/permissions answers. */
export type PermissionInfo = {
  key: import('./session.svelte').Permission
  name: string
  description: string
  /** Whether a Project's Permission override can set it. */
  overridable: boolean
}

export type Invitation = {
  id: number
  email: string
  /** The Roles it gives besides @everyone, top first. */
  roles: import('./session.svelte').RoleRef[]
  /** The former role those Roles read as. */
  role: 'admin' | 'member' | 'viewer'
  created_at: string
  expires_at: string
}

export type ChannelKind = 'email' | 'discord' | 'telegram' | 'slack' | 'pushover' | 'webhook' | 'ntfy'

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
    /** The display name in From:; empty means "The Bakery". */
    from_name?: string
    to?: string[]
    /** SMTP timeout in seconds; empty means 30. */
    timeout?: number
    ehlo_domain?: string
    /** ntfy's server; the other kinds' URLs are secret. */
    url?: string
    /** Of a secret URL, only its host. */
    url_host?: string
    /** Discord: mention @here on an alarming Event kind. */
    ping: boolean
    chat_id?: string
    has_bot_token: boolean
    /** Telegram: a forum topic (message thread id) per Event kind. */
    thread_ids?: Record<string, string>
    has_user_key: boolean
    has_api_token: boolean
    topic?: string
    has_token: boolean
    has_secret: boolean
  }
  event_kinds: string[]
  /** A disabled channel gets no Notifications and cannot be tested. */
  enabled: boolean
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

/** What an API token may do, as Coolify's Sanctum abilities. */
export type Permission = 'root' | 'write' | 'deploy' | 'read' | 'read:sensitive'

export type ApiToken = {
  id: number
  /** Coolify's "Description". */
  name: string
  permissions: Permission[]
  read_only: boolean
  created_at: string
  last_used_at: string | null
  /** null is Never. */
  expires_at: string | null
}
