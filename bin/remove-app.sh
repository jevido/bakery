#!/usr/bin/env bash
set -e

SUB=$1
ROOT=/srv/bakery
SERVICE=bakery-${SUB//./-}

sudo systemctl stop $SERVICE || true
sudo systemctl disable $SERVICE || true
sudo rm -f /etc/systemd/system/$SERVICE.service
sudo rm -rf $ROOT/apps/$SUB
sudo rm -rf /etc/letsencrypt/live/$SUB /etc/letsencrypt/archive/$SUB

echo "🗑️  Removed app $SUB and its cert"
