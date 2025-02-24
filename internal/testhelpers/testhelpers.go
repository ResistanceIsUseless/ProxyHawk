package testhelpers

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
)

// LoadProxies loads test proxy data from a file
func LoadProxies(filename string) ([]string, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(data), "\n")
	var proxies []string
	for _, line := range lines {
		if line = strings.TrimSpace(line); line != "" {
			proxies = append(proxies, line)
		}
	}
	return proxies, nil
}

// LoadConfig loads test configuration from a file
func LoadConfig(filename string) (map[string]interface{}, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}
	return config, nil
}

// CreateTestServer creates a test HTTP server for proxy testing
func CreateTestServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test response"))
	}))
}

// CheckProxyWithInteractsh performs a proxy check using a test Interactsh server
func CheckProxyWithInteractsh(proxyURL string) (bool, error) {
	// Mock implementation for testing
	return true, nil
}
