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
  public_url: string
}

export type Environment = { id: number; name: string; applications: Application[] }

export type Project = {
  id: number
  name: string
  description: string
  environments?: Environment[]
}

export type EnvVar = { name: string; value: string }

export type ApplicationInput = Pick<Application, 'name' | 'git_url' | 'git_branch' | 'dockerfile_path' | 'port' | 'domain'>

export type DeploymentStatus = 'queued' | 'cloning' | 'building' | 'starting' | 'finished' | 'failed'

export type Deployment = {
  id: number
  application_id: number
  status: DeploymentStatus
  active: boolean
  trigger: 'manual' | 'webhook'
  branch: string
  commit_sha: string
  commit_message: string
  commit_author: string
  image: string
  container: string
  error: string
  created_at: string
  started_at: string | null
  finished_at: string | null
}

export type LogLine = { stream: 'info' | 'out' | 'err'; line: string }
