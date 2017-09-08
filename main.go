package main

import (
	"./logging"
	"log"
	"os"
	"strings"
	//	"sync"
	//	"github.com/hashicorp/go-reap"
)

func main() {
	log.SetOutput(os.Stdout)
	if strings.EqualFold(os.Getenv("DEBUG"), "true") {
		go logging.StartLogging()
	}

	//	NOTE: Does not seem to reap zombie processes.
	//	TODO: Uncomment after fixing it
	//	pids := make(reap.PidCh, 2)
	//	errors := make(reap.ErrorCh, 2)
	//	done := make(chan struct{})
	//	var reapLock sync.RWMutex
	//	go reap.ReapChildren(pids, errors, done, &reapLock)

	// TODO: Change to serverImpl.Execute
	newArgs().parse()
}
