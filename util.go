package main

import (
	"io/ioutil"
	"log"
	"net"
	"net/http"
)

var readFile = ioutil.ReadFile
var httpListenAndServe = http.ListenAndServe
var httpWriterSetContentType = func(w http.ResponseWriter, value string) {
	w.Header().Set("Content-Type", value)
}
var logPrintf = log.Printf

var lookupHost = net.LookupHost
