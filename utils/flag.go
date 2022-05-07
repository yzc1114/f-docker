package utils

import (
	"fmt"
	flag "github.com/spf13/pflag"
	"log"
	"os"
)

func ParseSingleArg(msg string) string {
	fs := flag.FlagSet{}
	if err := fs.Parse(os.Args[2:]); err != nil {
		fmt.Println("Error parsing: ", err)
	}
	if len(fs.Args()) < 1 {
		log.Fatalf(msg)
	}
	return fs.Args()[0]
}