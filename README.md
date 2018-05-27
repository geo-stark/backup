Backup tool for linux and cloud storage
=================

Many companies propose some cloud storage for free and it can be used perfectly to backup private data. The good review of free cloud storage is here: https://www.thebalanceeveryday.com/free-cloud-storage-1356638. This backup tool supports flexible configuration for any number of backup paths with independent scheduling, cloud binding, file / folder excluding, compression, encryption. The tool automatically tracks backup scheduling so it can be put to cron to run one a day. For now Google Drive and Yandex Disk are supported only

## Installation
- (if needed) install google drive tool https://github.com/odeke-em/drive and init some folder
- (if needed) install yandex disk tool: https://github.com/abbat/ydcmd and configure it
- install gnupg from linux distributive's repository
- clone and compile program (no external dependencies are needed):

    go build .
    
- edit cloud-backup.ini file (see comments inside) and put it along with executable or in the root of home folder
- add it to /etc/crontab for every night running e.g.
    4 0  *  * * user_name /home/st/user/backup/cloud-backup

## Running
 - cloud-backup - check backup schedule and perform backup if needed
 - cloud-backup reset - reset backup state file
 - cloud-backup clear-archive - remove backup files from google drive
 - cloud-backup restore <path> - restore backup to the working directory

## Process
When program is executed it loads state file. State file contains data hash and date of last backup for every backup path.
Then it checks every path's backup period. If it is time that data at the path is compressed, compressed data's hash is compared to the hash from previous backup; if hashes don't match compressed data is encrypted and pushed to cloud

## License

This project is under MIT License. 


