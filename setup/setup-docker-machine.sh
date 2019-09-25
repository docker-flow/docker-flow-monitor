#!/bin/bash
export DB_BKP_FOLDER=~/db_backups
export LOG_BKP_FOLDER=~/db_backups

export DB_BKP_FOLDER=~/db_backups
export LOG_BKP_FOLDER=~/log_backups
export MIN_BKP_RETENTION=7
export BKP_FILE_ROTATION=2
export BKP_DAYS_ROTATION=$((MIN_BKP_RETENTION * BKP_FILE_ROTATION))

DAEMON_FILE="/etc/docker/daemon.json"
ROTATE_FILE="/etc/logrotate.d/docker"


sudo bash -c "cat > $DAEMON_FILE " <<EOF
{
"log-driver": "json-file",
"log-opts": {
    "max-size": "1m",    
    "max-file": "3",
    "compress": "true"
    }
} 
EOF

## This sohuld be executed at container level, not VM level
#   docker exec ... 
# sudo bash -c "cat > $ROTATE_FILE " <<EOF
# /var/lib/docker/containers/*/*.log {
#         rotate 2
#         weekly
#         compress
# }
# EOF
###############################################################################
##----- Cron jobs -----------
# -- Log BKP
(crontab -l 2>/dev/null; echo "0 0 * * 0 ./cron-backup-logs.sh") | crontab -

# -- DB BKP 
(crontab -l 2>/dev/null; echo "0 1 * * 0 ./cron-backup.sh") | crontab -
###############################################################################
## Sync time
sudo -i
date +%T -s "${CURRENT_TIME}"
###############################################################################


#### NOTES
# sudo find /var/lib/docker/containers -mtime -7 -type f -name "*json.log" -exec cp -a "{}" . \; 
# sudo find /var/lib/docker/containers -mtime +14 -type f -name *json.log -delete   
##---------------------------

