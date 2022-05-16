package utils

import "log"

func Must(err error) {
	if err != nil {
		log.Fatalf("Fatal error: %v\n", err)
	}
}

func MustWithMsg(err error, msg string) {
	if err != nil {
		log.Fatalf("Fatal error: %s: %v\n", msg, err)
	}
}
