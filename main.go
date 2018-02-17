package main

import (
	"log"
	"os"
	"strings"

	"./logging"
)

func main() {
	log.SetOutput(os.Stdout)
	if strings.EqualFold(os.Getenv("DEBUG"), "true") {
		go logging.StartLogging()
	}

	// TODO: Change to serverImpl.Execute
	newArgs().parse()
}
