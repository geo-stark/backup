package ext_logger

import (
	"fmt"
	"os"
)

type ExtLogger struct {
	file *os.File
}

func (this *ExtLogger) SetFile(file_name string) {
	if len(file_name) == 0 {
		return
	}
	var err error
	this.file, err = os.OpenFile(file_name, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		fmt.Printf("error opening ext log file: %v\n", err)
		os.Exit(1)
	}
}

func (this *ExtLogger) Close() {
	//fmt.Print("close ext logger\n")
	this.file.Close()
}

// io.Write interface
func (this *ExtLogger) Write(p []byte) (n int, err error) {
	//var e Error
	if this.file != nil {
		_, e := this.file.Write(p)
		if e != nil {
			fmt.Printf("%v", e)
		}
		this.file.Sync()
	}
	os.Stdout.Write(p)
	return 0, nil
}
