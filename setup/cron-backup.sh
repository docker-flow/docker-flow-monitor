#!/bin/bash
# remove bkps older than 14 days (keep 2 copies)
MONGO_IMAGE_ID=`docker ps --no-trunc -f status=running -f ancestor=mongo:3.4.1 | awk 'END{ print $NF }'`
if [ $MONGO_IMAGE_ID = "NAMES" ]; 
then 
    echo "No image found"; 
else 
    echo "Found Mongo image ${MONGO_IMAGE_ID}"; 
    docker run --rm --volumes-from ${MONGO_IMAGE_ID} -v $DB_BKP_FOLDER:/backup ubuntu tar cvf /backup/backup-$(date +%Y-%m-%d).tar /data/db

    sudo find $DB_BKP_FOLDER -mtime +$BKP_DAYS_ROTATION -type f -name *.tar -delete
fi



# docker run --rm --volumes-from ${MONGO_IMAGE_ID} -v $DB_BKP_FOLDER:/backup ubuntu tar cvf /backup-$(date +%Y-%m-%d).tar /data/db


