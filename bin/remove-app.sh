#!/usr/bin/env bash
set -e

SUB=$1
ROOT=/srv/bakery
SERVICE=bakery-${SUB//./-}

# Stop and disable systemd service
sudo systemctl stop $SERVICE || true
sudo systemctl disable $SERVICE || true
sudo rm -f /etc/systemd/system/$SERVICE.service

# Remove app files and SSL certs
sudo rm -rf $ROOT/apps/$SUB
sudo rm -rf /etc/letsencrypt/live/$SUB /etc/letsencrypt/archive/$SUB

# Drop Postgres database and user
sudo -u postgres psql <<EOF
DROP DATABASE IF EXISTS "$SUB";
DROP USER IF EXISTS "$SUB";
EOF

echo "🗑️  Removed app $SUB, its cert, database, and user"