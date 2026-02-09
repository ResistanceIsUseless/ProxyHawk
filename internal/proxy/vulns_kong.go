package proxy

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// KongVulnResult contains results from Kong API Gateway vulnerability checks
type KongVulnResult struct {
	ManagerExposed     bool     `json:"manager_exposed"`     // Kong Manager admin panel exposed
	KongaExposed       bool     `json:"konga_exposed"`       // Konga dashboard exposed
	AdminAPIExposed    bool     `json:"admin_api_exposed"`   // Admin API accessible
	UnauthorizedAccess bool     `json:"unauthorized_access"` // Can access config without auth
	ExposedEndpoints   []string `json:"exposed_endpoints,omitempty"`
	ExposedRoutes      []string `json:"exposed_routes,omitempty"`
	ExposedServices    []string `json:"exposed_services,omitempty"`
	ExposedConsumers   []string `json:"exposed_consumers,omitempty"`
}

// testKongManagerExposure tests for Kong Manager admin panel exposure
func (c *Checker) testKongManagerExposure(client *http.Client, result *ProxyResult) bool {
	if c.debug {
		result.DebugInfo += "[KONG MANAGER] Testing for Kong Manager exposure\n"
	}

	// Kong Manager default paths
	managerPaths := []string{
		"/",
		"/konga",
		"/kong-manager",
		"/admin",
		"/manager",
	}

	for _, path := range managerPaths {
		req, err := http.NewRequest("GET", c.config.ValidationURL+path, nil)
		if err != nil {
			continue
		}

		req.Header.Set("User-Agent", c.config.UserAgent)
		req.Header.Set("Accept", "text/html,application/xhtml+xml")

		resp, err := client.Do(req)
		if err != nil {
			continue
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		bodyStr := strings.ToLower(string(body))

		if resp.StatusCode == 200 {
			// Check for Kong Manager indicators
			if strings.Contains(bodyStr, "kong manager") || strings.Contains(bodyStr, "kong-manager") ||
				strings.Contains(bodyStr, "kong gateway") || strings.Contains(bodyStr, "kong admin") {
				if c.debug {
					result.DebugInfo += fmt.Sprintf("  [CRITICAL] Kong Manager exposed at: %s\n", path)
				}
				return true
			}

			// Check for Konga dashboard indicators
			if strings.Contains(bodyStr, "konga") || strings.Contains(bodyStr, "kong dashboard") {
				if c.debug {
					result.DebugInfo += fmt.Sprintf("  [CRITICAL] Konga dashboard exposed at: %s\n", path)
				}
				return true
			}
		}
	}

	return false
}

// testKongAdminAPI tests for Kong Admin API exposure
func (c *Checker) testKongAdminAPI(client *http.Client, result *ProxyResult) (bool, []string) {
	if c.debug {
		result.DebugInfo += "[KONG ADMIN API] Testing for Admin API exposure\n"
	}

	exposedEndpoints := []string{}

	// Kong Admin API endpoints
	apiTests := []struct {
		path string
		desc string
	}{
		{"/", "API root"},
		{"/status", "status endpoint"},
		{"/routes", "routes listing"},
		{"/services", "services listing"},
		{"/consumers", "consumers listing"},
		{"/plugins", "plugins listing"},
		{"/certificates", "certificates listing"},
		{"/upstreams", "upstreams listing"},
		{"/targets", "targets listing"},
	}

	for _, test := range apiTests {
		req, err := http.NewRequest("GET", c.config.ValidationURL+test.path, nil)
		if err != nil {
			continue
		}

		req.Header.Set("User-Agent", c.config.UserAgent)
		req.Header.Set("Accept", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			continue
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode == 200 {
			var jsonData map[string]interface{}
			if err := json.Unmarshal(body, &jsonData); err == nil {
				// Check for Kong API response structure
				if _, hasData := jsonData["data"]; hasData {
					exposedEndpoints = append(exposedEndpoints, fmt.Sprintf("%s (%s)", test.path, test.desc))
					if c.debug {
						result.DebugInfo += fmt.Sprintf("  [CRITICAL] Kong Admin API exposed: %s\n", test.path)
					}
				}

				// Check for status endpoint
				if test.path == "/status" {
					if _, hasDB := jsonData["database"]; hasDB {
						exposedEndpoints = append(exposedEndpoints, fmt.Sprintf("%s (%s)", test.path, test.desc))
						if c.debug {
							result.DebugInfo += fmt.Sprintf("  [CRITICAL] Kong status endpoint exposed: %s\n", test.path)
						}
					}
				}

				// Check for root endpoint
				if test.path == "/" {
					if version, ok := jsonData["version"]; ok {
						exposedEndpoints = append(exposedEndpoints, fmt.Sprintf("/ (Kong %v)", version))
						if c.debug {
							result.DebugInfo += fmt.Sprintf("  [CRITICAL] Kong API root exposed (version: %v)\n", version)
						}
					}
				}
			}
		}
	}

	return len(exposedEndpoints) > 0, exposedEndpoints
}

// testKongRoutesExposure tests for exposed Kong routes
func (c *Checker) testKongRoutesExposure(client *http.Client, result *ProxyResult) (bool, []string) {
	if c.debug {
		result.DebugInfo += "[KONG ROUTES] Testing for exposed routes configuration\n"
	}

	exposedRoutes := []string{}

	req, err := http.NewRequest("GET", c.config.ValidationURL+"/routes", nil)
	if err != nil {
		return false, nil
	}

	req.Header.Set("User-Agent", c.config.UserAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return false, nil
	}

	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	if resp.StatusCode == 200 {
		var routesResp struct {
			Data []map[string]interface{} `json:"data"`
		}

		if err := json.Unmarshal(body, &routesResp); err == nil {
			for _, route := range routesResp.Data {
				if name, ok := route["name"].(string); ok {
					exposedRoutes = append(exposedRoutes, name)
				} else if id, ok := route["id"].(string); ok {
					exposedRoutes = append(exposedRoutes, id)
				}
			}

			if len(exposedRoutes) > 0 {
				if c.debug {
					result.DebugInfo += fmt.Sprintf("  [HIGH] Exposed %d Kong routes\n", len(exposedRoutes))
				}
				return true, exposedRoutes
			}
		}
	}

	return false, nil
}

// testKongServicesExposure tests for exposed Kong services
func (c *Checker) testKongServicesExposure(client *http.Client, result *ProxyResult) (bool, []string) {
	if c.debug {
		result.DebugInfo += "[KONG SERVICES] Testing for exposed services configuration\n"
	}

	exposedServices := []string{}

	req, err := http.NewRequest("GET", c.config.ValidationURL+"/services", nil)
	if err != nil {
		return false, nil
	}

	req.Header.Set("User-Agent", c.config.UserAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return false, nil
	}

	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	if resp.StatusCode == 200 {
		var servicesResp struct {
			Data []map[string]interface{} `json:"data"`
		}

		if err := json.Unmarshal(body, &servicesResp); err == nil {
			for _, service := range servicesResp.Data {
				if name, ok := service["name"].(string); ok {
					exposedServices = append(exposedServices, name)
				} else if id, ok := service["id"].(string); ok {
					exposedServices = append(exposedServices, id)
				}
			}

			if len(exposedServices) > 0 {
				if c.debug {
					result.DebugInfo += fmt.Sprintf("  [HIGH] Exposed %d Kong services\n", len(exposedServices))
				}
				return true, exposedServices
			}
		}
	}

	return false, nil
}

// testKongConsumersExposure tests for exposed Kong consumers
func (c *Checker) testKongConsumersExposure(client *http.Client, result *ProxyResult) (bool, []string) {
	if c.debug {
		result.DebugInfo += "[KONG CONSUMERS] Testing for exposed consumers\n"
	}

	exposedConsumers := []string{}

	req, err := http.NewRequest("GET", c.config.ValidationURL+"/consumers", nil)
	if err != nil {
		return false, nil
	}

	req.Header.Set("User-Agent", c.config.UserAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return false, nil
	}

	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	if resp.StatusCode == 200 {
		var consumersResp struct {
			Data []map[string]interface{} `json:"data"`
		}

		if err := json.Unmarshal(body, &consumersResp); err == nil {
			for _, consumer := range consumersResp.Data {
				if username, ok := consumer["username"].(string); ok {
					exposedConsumers = append(exposedConsumers, username)
				} else if id, ok := consumer["id"].(string); ok {
					exposedConsumers = append(exposedConsumers, id)
				}
			}

			if len(exposedConsumers) > 0 {
				if c.debug {
					result.DebugInfo += fmt.Sprintf("  [CRITICAL] Exposed %d Kong consumers\n", len(exposedConsumers))
				}
				return true, exposedConsumers
			}
		}
	}

	return false, nil
}

// performKongVulnerabilityChecks runs all Kong API Gateway vulnerability checks
func (c *Checker) performKongVulnerabilityChecks(client *http.Client, result *ProxyResult) *KongVulnResult {
	kongResult := &KongVulnResult{}

	// Test Kong Manager exposure
	kongResult.ManagerExposed = c.testKongManagerExposure(client, result)
	if kongResult.ManagerExposed {
		kongResult.KongaExposed = true // Konga is detected in same check
	}

	// Test Admin API exposure
	var adminAPIExposed bool
	adminAPIExposed, kongResult.ExposedEndpoints = c.testKongAdminAPI(client, result)
	kongResult.AdminAPIExposed = adminAPIExposed

	// Test routes exposure
	var routesExposed bool
	routesExposed, kongResult.ExposedRoutes = c.testKongRoutesExposure(client, result)
	if routesExposed {
		kongResult.UnauthorizedAccess = true
	}

	// Test services exposure
	var servicesExposed bool
	servicesExposed, kongResult.ExposedServices = c.testKongServicesExposure(client, result)
	if servicesExposed {
		kongResult.UnauthorizedAccess = true
	}

	// Test consumers exposure
	var consumersExposed bool
	consumersExposed, kongResult.ExposedConsumers = c.testKongConsumersExposure(client, result)
	if consumersExposed {
		kongResult.UnauthorizedAccess = true
	}

	return kongResult
}
