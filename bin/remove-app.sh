#!/usr/bin/env bash
set -e

SUB=$1
ROOT=/srv/bakery
SERVICE=bakery-${SUB//./-}
DB_USER=bakery_${SUB//./_}


# Stop and disable systemd service
sudo systemctl stop $SERVICE || true
sudo systemctl disable $SERVICE || true
sudo rm -f /etc/systemd/system/$SERVICE.service

# Remove app files and SSL certs
sudo rm -rf $ROOT/apps/$SUB
sudo rm -rf /etc/letsencrypt/live/$SUB /etc/letsencrypt/archive/$SUB

# Drop Postgres database and user
sudo -u postgres psql <<EOF
-- Terminate connections
REVOKE CONNECT ON DATABASE "$DB_USER" FROM PUBLIC;
SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = '$DB_USER';

-- Drop database if exists
DROP DATABASE IF EXISTS "$DB_USER";

-- Remove schema privileges (safety)
REASSIGN OWNED BY "$DB_USER" TO postgres;
DROP OWNED BY "$DB_USER";

-- Now drop the role
DROP ROLE IF EXISTS "$DB_USER";
EOF

echo "🗑️  Removed app $SUB, its cert, database, and user"