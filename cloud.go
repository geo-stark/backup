package main

import (
	"os/exec"
)

type Cloud interface {	
	name() string
	command() string
	remove(item string) ([]byte, error)
	upload(localFile string, remoteFile string) ([]byte, error)
	download(remoteFile string) ([]byte, error)
}

type CloudGDrive struct { }

func (this CloudGDrive)name() string {
	return "Google Drive"
}

func (this CloudGDrive)command() string {
	return "drive"
}

func (this CloudGDrive)remove(remoteFileName string) ([]byte, error) {
	var content = "drive delete -quiet " + remoteFileName
	var cmd = exec.Command("sh", "-c", content)
	return cmd.CombinedOutput();
}

// item.archive, options.remote_folder
func (this CloudGDrive)upload(localFile string, remotePath string) ([]byte, error) {
	var content = "drive push -quiet " + localFile
	var cmd = exec.Command("sh", "-c", content)
	return cmd.CombinedOutput();
}

func (this CloudGDrive)download(remotePath string) ([]byte, error) {
	var content = "drive pull -quiet " + remotePath
	var cmd = exec.Command("sh", "-c", content)
	return cmd.CombinedOutput();
}

type CloudYDisk struct { }

func (this CloudYDisk)remove(remoteFileName string) ([]byte, error) {
	var content = "ydcmd rm " + remoteFileName
	var cmd = exec.Command("sh", "-c", content)
	return cmd.CombinedOutput();
}

// item.archive, options.remote_folder
func (this CloudYDisk)upload(localFile string, remotePath string) ([]byte, error) {
	var content = "ydcmd put " + localFile + "  " + remotePath
	var cmd = exec.Command("sh", "-c", content)
	return cmd.CombinedOutput();
}

func (this CloudYDisk)download(remotePath string) ([]byte, error) {
	var content = "ydcmd get --quiet " + remotePath
	var cmd = exec.Command("sh", "-c", content)
	return cmd.CombinedOutput();
}

func (this CloudYDisk)name() string {
	return "Yandex Disk"
}

func (this CloudYDisk)command() string {
	return "ydcmd"
}

//------------------------------------------------------------------------------
func getCloudByName(name string) Cloud { 
	switch name {
	case "gdrive":
		return CloudGDrive{};
	case "ydisk": 
		return CloudYDisk{};
	}	
	return nil
}


