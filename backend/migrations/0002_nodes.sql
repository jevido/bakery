CREATE TABLE IF NOT EXISTS nodes (
  id TEXT PRIMARY KEY,
  owner_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'pending',
  install_token TEXT,
  api_token TEXT,
  pairing_code TEXT,
  last_seen TIMESTAMP WITH TIME ZONE,
  metadata JSONB DEFAULT '{}'::jsonb,
  created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
  updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_nodes_install_token
  ON nodes (install_token)
  WHERE install_token IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_nodes_api_token
  ON nodes (api_token)
  WHERE api_token IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_nodes_pairing_code
  ON nodes (pairing_code)
  WHERE pairing_code IS NOT NULL;

ALTER TABLE deployments
  ADD COLUMN IF NOT EXISTS node_id TEXT REFERENCES nodes(id);

ALTER TABLE tasks
  ADD COLUMN IF NOT EXISTS node_id TEXT REFERENCES nodes(id),
  ADD COLUMN IF NOT EXISTS reserved_by TEXT;

CREATE INDEX IF NOT EXISTS idx_tasks_node_status ON tasks (node_id, status, created_at);
