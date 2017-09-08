package server

import (
	"fmt"
	"github.com/gorilla/schema"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
)

var httpWriterSetContentType = func(w http.ResponseWriter, value string) {
	w.Header().Set("Content-Type", value)
}
var logPrintf = log.Printf
var lookupHost = net.LookupHost

var decoder = schema.NewDecoder()

var sendDistributeRequests = func(req *http.Request, port, proxyServiceName string) (status int, err error) {
	values := req.URL.Query()
	values.Set("distribute", "false")
	req.URL.RawQuery = values.Encode()
	dns := fmt.Sprintf("tasks.%s", proxyServiceName)
	failedDns := []string{}
	method := req.Method
	body := ""
	if req.Body != nil {
		defer func() { req.Body.Close() }()
		reqBody, _ := ioutil.ReadAll(req.Body)
		body = string(reqBody)
	}
	if ips, err := lookupHost(dns); err == nil {
		if len(ips) == 0 {
			err := fmt.Errorf(
				`Could not resend distribution requests since no replicas of the "%s" were found. Please check that the name of the service is "%s". If it isn't, set the environment variable "SERVICE_NAME" with the name of the proxy service as value.`,
				proxyServiceName,
				proxyServiceName,
			)
			return http.StatusBadRequest, err
		}
		for i := 0; i < len(ips); i++ {
			req.URL.Host = fmt.Sprintf("%s:%s", ips[i], port)
			client := &http.Client{}
			addr := fmt.Sprintf("http://%s:%s%s?%s", ips[i], port, req.URL.Path, req.URL.RawQuery)
			logPrintf("Sending distribution request to %s", addr)
			req, _ := http.NewRequest(method, addr, strings.NewReader(body))
			if resp, err := client.Do(req); err != nil || resp.StatusCode >= 300 {
				failedDns = append(failedDns, ips[i])
			}
		}
	} else {
		err := fmt.Errorf(
			"Could not perform DNS lookup for %s. If the proxy is not called 'proxy', you must set SERVICE_NAME=<name-of-the-proxy> on the proxy service.",
			dns,
		)
		return http.StatusBadRequest, err
	}
	if len(failedDns) > 0 {
		err := fmt.Errorf("Could not send distribute request to the following addresses: %s", failedDns)
		return http.StatusBadRequest, err
	}
	return http.StatusOK, err
}

var getSecretOrEnvVar = func(key, defaultValue string) string {
	path := fmt.Sprintf("/run/secrets/dfp_%s", strings.ToLower(key))
	if content, err := readSecretsFile(path); err == nil {
		return strings.TrimRight(string(content[:]), "\n")
	}
	if len(os.Getenv(key)) > 0 {
		return os.Getenv(key)
	}
	return defaultValue
}
var readSecretsFile = ioutil.ReadFile
