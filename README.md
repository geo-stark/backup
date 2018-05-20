Backup tool for linux and google drive
=================

As an owner of google account I've got 15GB of disk space in Google drive for free and it can be used perfectly to backup private program code documents etc
The reason for writing my own backup tool was learning the go programming language and control backup process the way I prefer

## Installation
- install google drive tool https://github.com/odeke-em/drive and init some folder
- install gnupg from linux distributive's repository
- clone and compile program (no external dependencies are needed):
    go build backup.go
- edit backup.ini file and put it along with executable
- add it to /etc/crontab for every night running e.g.
    4 0  *  * * user_name /home/st/user/backup/backup

## Running
 - backup - check backup schedule and perform backup if needed
 - backup reset - reset backup state file
 - backup clear-archive - remove backup files from google drive
 - backup restore <path> - restore backup to the working directory

## Process
See backup.ini comments
When program is executed it loads state file. State file contains hash and date of last backup for every backup path.
Then it checks every path's backup period. If it is time that data at the path is compressed, compressed data's hash is compared to the hash from previous backup; if hashes don't match 
compressed data is encrypted and pushed to google drive

## License

This project is under MIT License. 


