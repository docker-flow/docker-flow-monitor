#!/bin/bash

mkdir -p $LOG_BKP_FOLDER/tmp
rm $LOG_BKP_FOLDER/tmp/*

sudo find /var/lib/docker/containers -mtime -$MIN_BKP_RETENTION -type f -name "*json.log" -exec cp -a "{}" $LOG_BKP_FOLDER/tmp \; 
sudo tar --remove-files -zcvf $LOG_BKP_FOLDER/log-bkp-$(date +%Y-%m-%d).tar $LOG_BKP_FOLDER/tmp 

# removes old ones
sudo find $LOG_BKP_FOLDER -mtime +$BKP_DAYS_ROTATION -type f -name *.tar -delete