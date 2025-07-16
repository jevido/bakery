#!/bin/bash
CRON_TMP="/tmp/bakery-cron.$$.tmp"
> "$CRON_TMP"

for app in /srv/bakery/apps/*; do
    if [ -d "$app/current/cron" ]; then
        for job in "$app"/current/cron/*.js; do
            schedule=$(grep -m1 '^// schedule:' "$job" | sed 's|// schedule: ||')
            if [ -n "$schedule" ]; then
                echo "$schedule bun run $job >> /var/log/cron-$(basename "$app").log 2>&1" >> "$CRON_TMP"
            fi
        done
    fi
done

crontab "$CRON_TMP"
rm "$CRON_TMP"