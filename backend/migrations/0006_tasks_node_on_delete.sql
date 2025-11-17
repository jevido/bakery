ALTER TABLE tasks
DROP CONSTRAINT IF EXISTS tasks_node_id_fkey;

ALTER TABLE tasks
ADD CONSTRAINT tasks_node_id_fkey
FOREIGN KEY (node_id)
REFERENCES nodes(id)
ON DELETE SET NULL;
