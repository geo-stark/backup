//------------------------------------------------------------------------------
// File        : backup.go
// Author      : George Stark (george-u@yandex.com)
// Created on  : May 19, 2018
// License     : MIT
//------------------------------------------------------------------------------
package main

import (
	"bufio"
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"errors"

	"./libs/ext-logger"
	"./libs/github.com/go-ini/ini"
	"./libs/github.com/cloudfoundry/bytefmt"
)

const configFile = "cloud-backup.ini"

const (
	Once    = iota
	Dayly   = iota
	Weekly  = iota
	Monthly = iota
)

type PathItem struct {
	path        string
	exclude     []string
	pathHash    string
	encryption  bool
	compression bool
	dataHash    string
	schedule    int
	date        time.Time
	archive     string		// base name
	archiveSize int64
	upload      bool
	cloud 		Cloud
}

type Options struct {
	logFile     string
	stateFile   string
	workingPath string
	password    string
	weeklyDays  []int
	monthlyDays []int
	cloudName	string
	cloudPath	string
	level		int
	verbose		bool
}

//------------------------------------------------------------------------------
func changeDirectory(path string) { 
	log.Printf("switch to %s\n", path)
	os.Chdir(path)
}

//------------------------------------------------------------------------------
func checkCommandExists(command string) bool {
	cmd := exec.Command("sh", "-c", "hash " + command)
	return cmd.Run() == nil
}

//------------------------------------------------------------------------------
func normalizePath(path string) string {
	cmd := exec.Command("sh", "-c", "realpath " + path)
	output, _ := cmd.Output()
	path = strings.Trim(string(output), "\r\n")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		log.Fatalf("path %s not exists\n", path)
	}
	return path
}

//------------------------------------------------------------------------------
func normalizePathNoCheck(path string) string {
	cmd := exec.Command("sh", "-c", "realpath " + path)
	output, _ := cmd.Output()
	return strings.Trim(string(output), "\r\n")
}

//------------------------------------------------------------------------------
func getStrHash(str string) string {
	hash := md5.Sum([]byte(str))
	return hex.EncodeToString(hash[:])
}

//------------------------------------------------------------------------------
func getFileHash(fileName string) string {
	file, err := os.Open(fileName)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	h := md5.New()
	if _, err := io.Copy(h, file); err != nil {
		log.Fatal(err)
	}
	hash := h.Sum(nil)
	return hex.EncodeToString(hash[:])
}

//------------------------------------------------------------------------------
func getList(input string, sep string) []string {
	var result []string

	for _, item := range strings.Split(input, sep) {
		if str := strings.Trim(item, " \t"); len(str) > 0 {
			result = append(result, str)
		}
	}
	return result
}

//------------------------------------------------------------------------------
func readDays(days []string) ([]int, error) {
	var result []int
	if len(days) == 0 {
		return nil, nil
	}
	for _, item := range days {
		var day = strings.ToLower(item)
		for n := time.Sunday; n <= time.Saturday; n++ {
			if strings.Index(strings.ToLower(n.String()), day) == 0 {
				result = append(result, int(n))
				break
			}
		}
	}
	return result, nil
}

//------------------------------------------------------------------------------
func loadOptions(values map[string]string) (Options, error) {
	var options Options
	var err error

	options.logFile = normalizePathNoCheck(values["log-file"])
	options.stateFile = normalizePathNoCheck(values["state-file"])
	options.workingPath = normalizePath(values["working-dir"])

	if len(options.workingPath) == 0 {
		log.Fatalln("working path is not specified")
	}
	options.workingPath += "/"

	options.password = values["password"]

	var days []string
	days = getList(values["monthly"], ",")
	for _, item := range days {
		n, _ := strconv.Atoi(item)
		options.monthlyDays = append(options.monthlyDays, n)
	}
	days = getList(values["weekly"], ",")
	options.weeklyDays, err = readDays(days)

	options.level = 2
	if len(values["compression-level"]) > 0 {
		options.level,_ = strconv.Atoi(values["compression-level"])
		if options.level < 0 || options.level > 9 {
			log.Fatalln("bad compression level value")
		}
	}
	
	options.cloudName = values["cloud"]
	options.cloudPath = values["cloud-dir"]
	if len(options.cloudPath) != 0 && options.cloudPath[len(options.cloudPath) - 1] != '/' {
		options.cloudPath += "/";
	}
	return options, err
}

//------------------------------------------------------------------------------
func loadPaths(values map[string]string, options Options) ([]PathItem, error) {
	var list []PathItem

	for path, value := range values {
		var item PathItem
		item.path = normalizePath(path)
		item.pathHash = getStrHash(path)
		item.archive = item.pathHash + ".bin"
		item.compression = true
		item.encryption = len(options.password) > 0

		for _, opt := range getList(value, ",") {
			switch opt {
			case "once":
				item.schedule = Once
			case "dayly":
				item.schedule = Dayly
			case "weekly":
				item.schedule = Weekly
			case "monthly":
				item.schedule = Monthly
			case "no-compression":
				item.compression = false
			case "no-encryption":
				item.encryption = false
			default:
				if strings.Index(opt, "exclude:") == 0 {
					item.exclude = getList(opt[len("exclude:"):], ":")
					break
				}
				if item.cloud = getCloudByName(opt); item.cloud == nil {
					log.Fatalf("unknown option %s, path %s \n", opt, path)
				}
			}
		}
		
		if item.cloud == nil {
			if item.cloud = getCloudByName(options.cloudName); item.cloud == nil {
				log.Fatalf("cloud name for path %s not specified\n", path);
			}
		}
		list = append(list, item)
	}
	return list, nil
}

//------------------------------------------------------------------------------
func loadState(fileName string, items []PathItem) error {
	// state file format
	// path:md5(path):md5(data):last backup date:archive size
	file, err := os.Open(fileName)
	if err != nil {
		return nil
	}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		list := strings.Split(scanner.Text(), ",")
		if len(list) < 5 {
			return nil
		}
		for index := range items {
			if items[index].pathHash == list[1] {
				items[index].dataHash = list[2]
				items[index].date.UnmarshalText([]byte(list[3]))
				items[index].archiveSize, _ = strconv.ParseInt(list[4], 10, 64)
				break
			}
		}
	}
	file.Close()
	return nil
}

//------------------------------------------------------------------------------
func saveState(fileName string, items []PathItem) error {
	file, err := os.Create(fileName)
	if err != nil {
		return err
	}
	for _, item := range items {
		date, _ := item.date.MarshalText()
		fmt.Fprintf(file, "%s,%s,%s,%s,%d\n", 
			item.path, item.pathHash, item.dataHash, string(date), item.archiveSize)
	}
	file.Close()
	return nil
}

//------------------------------------------------------------------------------
func getTotalBackupSize(items []PathItem) {
	var totalSize uint64
	var cloudSize = make(map[string]uint64);
	for index := range items {
		var size = uint64(items[index].archiveSize)
		totalSize += size
		cloudSize[items[index].cloud.name()] += size 
	}
	log.Printf("Total backup size for now: %s\n", bytefmt.ByteSize(totalSize))
	for name,size := range cloudSize {
		log.Printf("  %s size: %s\n", name, bytefmt.ByteSize(size))
	}
}

//------------------------------------------------------------------------------
func contain(list []int, value int) bool {
	for _, n := range list {
		if n == value {
			return true
		}
	}
	return false
}

//------------------------------------------------------------------------------
func logCommandOuput(output []byte) {
	str := strings.Replace(string(output), "\n", "|", -1)
	log.Printf("command output: %s\n", str)		
}

//------------------------------------------------------------------------------
func deleteArchive(item PathItem, options Options) {
	log.Printf("delete remote archive %s\n", item.archive)

	if output, err := item.cloud.remove(options.cloudPath + item.archive); err != nil {
		logCommandOuput(output)
		log.Printf("remote delete failed %v\n", err)		
	}
}

//------------------------------------------------------------------------------
func restoreArchive(item PathItem, options Options) error {

	changeDirectory(options.workingPath)
	os.Remove(item.archive)

	log.Printf("download %s\n", item.archive)
	if output,err := item.cloud.download(options.cloudPath + item.archive); err != nil {
		logCommandOuput(output)
		log.Printf("download archive failed %v\n", err)
		return err
	}
	
	var content string
	if item.encryption {
		content = "gpg -d  -o- --passphrase '" + options.password +  
		"' "  + item.archive
	} else {
		content = "cat " + item.archive
	} 

	content += " | tar xJ"
	cmd := exec.Command("sh", "-c", content)
	if _, err := cmd.Output(); err != nil {
		return err
	}
	os.Remove(item.archive)
	return nil
}

//------------------------------------------------------------------------------
func uploadArchive(item *PathItem, options Options) error {
	changeDirectory(options.workingPath)
	deleteArchive(*item, options)

	log.Printf("upload %s\n", item.archive)
	log.Printf("  encryption: %v\n", item.encryption)
	log.Printf("  cloud: %s\n", item.cloud.name())
	
	if output,err := item.cloud.upload(item.archive, options.cloudPath); err != nil {
		logCommandOuput(output)
		return err
	}
	os.Remove(item.archive)
	return nil
}

//------------------------------------------------------------------------------
func createArchive(item *PathItem, options Options) (bool, error) {

	var targetFile = options.workingPath + item.archive
	var buffer bytes.Buffer
	buffer.WriteString("tar")
	for _, s := range item.exclude {
		buffer.WriteString(" --exclude=")
		buffer.WriteString(s)
	}
	buffer.WriteString(" --mtime=0")
	buffer.WriteString(" -cf - ")
	buffer.WriteString(filepath.Base(item.path))
	buffer.WriteString(" | tee >")
	if item.compression || item.encryption  {
		buffer.WriteString("(")
		if item.compression {
			buffer.WriteString("xz --stdout -")
			buffer.WriteString(strconv.Itoa(options.level))
			if item.encryption  {
				buffer.WriteString(" - | ")
			} else {
				buffer.WriteString(" > ")
				buffer.WriteString(targetFile)
			}
		}
		if item.encryption {
			buffer.WriteString("gpg -z 0 -o ")
			buffer.WriteString(targetFile)
			buffer.WriteString(" --passphrase ")
			buffer.WriteString(options.password)
			buffer.WriteString(" -c - ")
		}
		buffer.WriteString(")")
	} else {
		buffer.WriteString(targetFile)
	}
	buffer.WriteString(" | md5sum")
	//tar --mtime=0 -cf - 'input' | tee >(xz --stdout - | gpg -z 0 -o 'output' --passphrase 'password' -c -) | md5sum
	
	os.Remove(targetFile)
	changeDirectory(filepath.Dir(item.path))	

	log.Printf("archive %s -> %s\n", item.path, targetFile)
	if options.verbose {
		log.Printf("command: %s\n", buffer.String())	
	}

	var output []byte	
	var err error
	cmd := exec.Command("bash", "-c", buffer.String())
	if output,err = cmd.CombinedOutput(); err != nil {
		return false, err
	}

	if len(output) < 32 {
		return false, errors.New("calculate md5 sum failed")
	}
	var hash = string(output[0:32]);

	fi, _ := os.Stat(targetFile)
	item.archiveSize = fi.Size()
	log.Printf("  size: %s\n", bytefmt.ByteSize(uint64(fi.Size())))
	log.Printf("  data hash: %s\n", hash)
	if hash == item.dataHash {
		log.Printf("source not changed, skipping ")
		os.Remove(targetFile)
		return false, nil
	}
	log.Printf("previous hash (%s) is different, mark to upload\n", item.dataHash)
	item.dataHash = hash
	return true, nil
}

//------------------------------------------------------------------------------
func proccessPathItem(item *PathItem, options Options) (bool, error) {
	log.Printf("proccessing path %s\n", item.path)
	var need bool
	var current = time.Now()

	switch item.schedule {
	case Once:
		need = item.date.IsZero()
	case Dayly:
		need = current.YearDay() != item.date.YearDay()
	case Weekly:
		if item.date.IsZero() || 
			(current.Day() != item.date.Day() &&
			contain(options.weeklyDays, int(current.Weekday()))) {
			need = true
		}
		break
	case Monthly:
		if item.date.IsZero() || 
			(current.Day() != item.date.Day() && 
			contain(options.monthlyDays, current.Day())) {
			need = true
		}
	}
	if !need {
		log.Printf("recently backuped, skipping\n")
		return false, nil
	}

	var err error
	log.Printf("back up %s\n", item.path)
	if item.upload, err = createArchive(item, options); err != nil {
		log.Printf("create archive failed %v", err)
		return false, err
	}
	if !item.upload {
		return false, nil
	}

	if err := uploadArchive(item, options); err != nil {
		log.Printf("upload archive failed %v", err)
		return false, err
	}
	return true, nil
}

//------------------------------------------------------------------------------
func parseCommandLine(paths []PathItem, options Options) {
	if len(os.Args) <= 1 {
		return
	}
	switch os.Args[1] {
	case "reset":
		log.Println("reset backup state")
		os.Remove(options.stateFile)
		os.Exit(0)
	case "clear-archive":
		log.Println("clear backup archive")
		changeDirectory(options.workingPath)
		for _, item := range paths {
			deleteArchive(item, options)
		}
		os.Exit(0)
	case "restore": 
		if len(os.Args) > 2 {
			var path = normalizePath(os.Args[2])
			log.Printf("restore path %s to working folder\n", path)
			for _, item := range paths {
				if item.path == path {
					if err := restoreArchive(item, options); err != nil {
						log.Fatalf("restore failed: %v", err)
					}
					log.Println("restored")
				}				
			}
			os.Exit(0)
		}
	}
	fmt.Println("commands: reset, clear-archive, restore <path>")	
	os.Exit(0)
}

//------------------------------------------------------------------------------
func checkCommands(paths []PathItem, options Options) {
	var commands = make(map[string]bool);
	commands["bash"] = true
	commands["tar"] = true
	commands["md5sum"] = true
	
	for _,item := range paths {
		if item.encryption {
			commands["gpg"] = true
		}
		if item.compression {
			commands["xz"] = true
		}
		commands[item.cloud.command()] = true
	}
	for item,_ := range commands {
		if !checkCommandExists(item) {
			log.Fatalf("required command %s not found\n", item)		
		}
	}
}

//------------------------------------------------------------------------------
func main() {
	var err error
	var logger ext_logger.ExtLogger
	log.SetOutput(&logger)
	defer logger.Close()

	var configPath = filepath.Dir(os.Args[0]) + "/" + configFile
	if _, err := os.Stat(configPath); err != nil {	
		configPath = normalizePathNoCheck("~/" + configFile)
	}
	log.Printf("open config file %s...\n", configPath)
	var cfg *ini.File
	if cfg, err = ini.Load(configPath); err != nil {
		log.Fatalf("config file not loaded: %v", err)
	}

	var options Options
	if options, err = loadOptions(cfg.Section("config").KeysHash()); err != nil {
		log.Fatalf("failed to read options: %v", err)
	}
	options.verbose = false

	logger.SetFile(options.logFile)
	log.Println("")
	log.Println("start logging")

	var paths []PathItem
	paths, err = loadPaths(cfg.Section("paths").KeysHash(), options)
	loadState(options.stateFile, paths)

	checkCommands(paths, options)
	parseCommandLine(paths, options)

	var backuped bool
	for index, item := range paths {
		if backuped, err = proccessPathItem(&item, options); err != nil {
			log.Printf("backup error for path %s: %v\n", item.path, err)
			continue
		}
		if backuped {
			item.date = time.Now()
			paths[index] = item
			if err = saveState(options.stateFile, paths); err != nil {
				log.Fatalf("error saving state: %v\n", err)
			}
		}
	}
	getTotalBackupSize(paths);
	log.Println("done")
}

