package main

import (
	"./logging"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"

	"github.com/hashicorp/go-reap"
)

func main() {
	log.SetOutput(os.Stdout)
	if strings.EqualFold(os.Getenv("DEBUG"), "true") {
		go logging.StartLogging()
	}
	// Get feedback on reaped children and errors.
	if reap.IsSupported() {
		pids := make(reap.PidCh, 2)
		errors := make(reap.ErrorCh, 2)
		done := make(chan struct{})
		var reapLock sync.RWMutex
		go reap.ReapChildren(pids, errors, done, &reapLock)
	} else {
		fmt.Println("Sorry, go-reap isn't supported on your platform.")
	}
	// TODO: Change to serverImpl.Execute
	NewArgs().Parse()
}
