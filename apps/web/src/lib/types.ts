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
