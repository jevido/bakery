ALTER TABLE deployments
ADD COLUMN IF NOT EXISTS dockerfile_path TEXT DEFAULT 'Dockerfile',
ADD COLUMN IF NOT EXISTS build_context TEXT DEFAULT '.';

UPDATE deployments
SET dockerfile_path = COALESCE(dockerfile_path, 'Dockerfile'),
	build_context = COALESCE(build_context, '.');
