package server

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
)

// TODO: Test with haproxy_exporter and Prometheus
// NOTE: It does not work until the first service is added
// TODO: Check whether data should be aggregated
// TODO: Document

type metricer interface {
	Get(w http.ResponseWriter, req *http.Request)
	getMetricsUrl() string
}

type metrics struct {
	metricsUrl string
}

// NewMetrics returns metricer instance
func NewMetrics(metricsUrl string) metricer {
	if len(metricsUrl) == 0 {
		statsUri := getSecretOrEnvVar(os.Getenv("STATS_URI_ENV"), "/admin?stats")
		statsPort := getSecretOrEnvVar("STATS_PORT", "")
		if len(statsPort) > 0 {
			statsPort = ":" + statsPort
		}
		metricsUrl = fmt.Sprintf("http://%slocalhost%s%s;csv", GetCreds(), statsPort, statsUri)
	}
	return &metrics{metricsUrl: metricsUrl}
}

func (m *metrics) getMetricsUrl() string {
	return m.metricsUrl
}

func (m *metrics) Get(w http.ResponseWriter, req *http.Request) {
	contentType := "text/html"
	if strings.EqualFold(req.URL.Query().Get("distribute"), "true") {
		dns := fmt.Sprintf("tasks.%s", os.Getenv("SERVICE_NAME"))
		ips, err := lookupHost(dns)
		if err != nil {
			logPrintf(err.Error())
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			body, err := m.getAllHaProxyMetrics(req, ips)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
			}
			w.Write(body)
		}
	} else {
		body, err := m.getHaProxyMetrics()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
		}
		w.Write(body)
	}
	httpWriterSetContentType(w, contentType)
}

func (m *metrics) getHaProxyMetrics() ([]byte, error) {
	resp, err := http.Get(m.metricsUrl)
	if err != nil {
		logPrintf("Failed to fetch metrics from %s\nERROR: %s", m.metricsUrl, err.Error())
		return []byte(""), err
	}
	defer resp.Body.Close()
	return ioutil.ReadAll(resp.Body)
}

func (m *metrics) getAllHaProxyMetrics(req *http.Request, ips []string) ([]byte, error) {
	msg := []byte("")
	for _, ip := range ips {
		values := req.URL.Query()
		values.Set("distribute", "false")
		req.URL.RawQuery = values.Encode()
		port := ""
		if !strings.Contains(ip, ":") {
			port = ":8080"
		}
		addr := fmt.Sprintf("http://%s%s/v1/docker-flow-proxy/metrics?%s", ip, port, req.URL.RawQuery)
		resp, err := http.Get(addr)
		if err != nil {
			logPrintf("Failed to fetch metrics from %s\nERROR: %s", addr, err.Error())
			return []byte(""), err
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 300 {
			return []byte(""), fmt.Errorf("Got response status %d", resp.StatusCode)
		}
		body, _ := ioutil.ReadAll(resp.Body)
		msg = append(msg, body...)
		if !bytes.HasSuffix(msg, []byte("\n")) {
			msg = append(msg, byte('\n'))
		}
	}
	return msg, nil
}

// GetCreds returns credentials from environment variables.
func GetCreds() string {
	statsUser := getSecretOrEnvVar(os.Getenv("STATS_USER_ENV"), "")
	statsPass := getSecretOrEnvVar(os.Getenv("STATS_PASS_ENV"), "")
	if len(statsUser) > 0 && !strings.EqualFold(statsUser, "none") && len(statsPass) > 0 && !strings.EqualFold(statsPass, "none") {
		return fmt.Sprintf("%s:%s@", statsUser, statsPass)
	}
	return ""
}
