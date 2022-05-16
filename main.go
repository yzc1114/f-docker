//go:build linux
// +build linux

package main

import (
	"fdocker/cmds"
	"fdocker/utils"
	"fdocker/workdirs"
	"fmt"
	"log"
	"math/rand"
	"os"
	"time"
)

func init() {
	rand.Seed(time.Now().UnixNano())
	utils.Must(workdirs.Init())
	if os.Geteuid() != 0 {
		log.Fatal("You need root privileges to run this program.")
	}
}

func main() {
	executors := cmds.GetCmdExecutors()
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}
	cmd := os.Args[1]
	if exec, ok := executors[cmd]; ok {
		exec.Exec()
	} else {
		usage()
	}
}

func usage() {
	fmt.Println("Usgae: ")
	usages := cmds.Usage()
	for _, usage := range usages {
		fmt.Println("\t" + usage)
	}
}
