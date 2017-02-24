package proxy

import (
	"io/ioutil"
	"log"
	"os/exec"
	"fmt"
	"strings"
	"os"
)

var cmdRunHa = func(cmd *exec.Cmd) error {
	return cmd.Run()
}
var readConfigsFile = ioutil.ReadFile
var readSecretsFile = ioutil.ReadFile
var writeFile = ioutil.WriteFile
var ReadFile = ioutil.ReadFile
var logPrintf = log.Printf
var readPidFile = ioutil.ReadFile
var readConfigsDir = ioutil.ReadDir
var GetSecretOrEnvVar = func(key, defaultValue string) string {
	path := fmt.Sprintf("/run/secrets/dfp_%s", strings.ToLower(key))
	if content, err := readSecretsFile(path); err == nil {
		return strings.TrimRight(string(content[:]), "\n")
	}
	if len(os.Getenv(key)) > 0 {
		return os.Getenv(key)
	}
	return defaultValue
}
