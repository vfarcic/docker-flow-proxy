package actions

import (
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
)

type executable interface {
	Execute(args []string) error
}

var lookupHost = net.LookupHost
var logPrintf = log.Printf
var httpGet = http.Get
var writeFeTemplate = ioutil.WriteFile
var writeBeTemplate = ioutil.WriteFile
var readTemplateFile = ioutil.ReadFile
var osRemove = os.Remove
