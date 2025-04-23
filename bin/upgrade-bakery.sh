#!/usr/bin/env bash
set -e
# Pull latest bakery repo & re-run setup
cd /srv/bakery/
sudo git pull
sudo bash setup-bakery.sh
echo "🔄 Bakery platform upgraded!"
