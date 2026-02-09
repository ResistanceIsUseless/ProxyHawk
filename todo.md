# ProxyHawk - Proxy Vulnerability Testing Implementation TODO

**Created:** 2026-02-09
**Status:** Research Complete - Ready for Implementation

## Priority 1: Critical Vulnerabilities (Week 1)

- [ ] **Infrastructure Setup**
  - [ ] Add proxy vulnerability check framework to `internal/proxy/advanced_checks.go`
  - [ ] Update configuration schema in `internal/config/config.go` for proxy vulnerability checks
  - [ ] Add YAML configuration section in `config/default.yaml`
  - [ ] Create test fixtures for vulnerable environments

- [ ] **CVE-2021-40438: Apache mod_proxy SSRF**
  - [ ] Implement detection function `checkApacheModProxySSRF()`
  - [ ] Add OAST (Interactsh) integration for blind SSRF detection
  - [ ] Test with Apache 2.4.48 and earlier
  - [ ] Add unit tests

- [ ] **CVE-2020-11984: Apache mod_proxy_uwsgi RCE**
  - [ ] Implement detection function `checkApacheModProxyUwsgiRCE()`
  - [ ] Build uwsgi protocol payload generator
  - [ ] Add OAST callback detection
  - [ ] Add unit tests with safety guards (avoid actual RCE in tests)

- [ ] **CVE-2024-38473: Apache mod_proxy ACL Bypass**
  - [ ] Implement detection function `checkApacheACLBypass()`
  - [ ] Test path normalization bypass (`%3f` technique)
  - [ ] Test DocumentRoot confusion
  - [ ] Add unit tests

- [ ] **Open Proxy to Localhost**
  - [ ] Implement detection function `checkOpenProxyLocalhost()`
  - [ ] Test HTTP/HTTPS access to 127.0.0.1 and localhost
  - [ ] Port scanning capability for common internal ports (8080, 9200, 6379)
  - [ ] Add fingerprint detection (IIS, Apache, Elasticsearch, Redis)

- [ ] **Kong Manager Exposure**
  - [ ] Implement detection function `checkKongManagerExposure()`
  - [ ] Test admin API endpoints (/routes, /services, /consumers)
  - [ ] Add Kong version detection
  - [ ] Add unit tests

## Priority 2: High-Impact Vulnerabilities (Week 2)

- [ ] **Nginx Off-by-Slash Path Traversal**
  - [ ] Implement detection function `checkNginxOffBySlash()`
  - [ ] Test common alias paths (/static, /js, /images, /assets, /css)
  - [ ] Git config detection logic
  - [ ] Add unit tests

- [ ] **Kubernetes API Exposure via Ingress Headers**
  - [ ] Implement detection function `checkK8sIngressExposure()`
  - [ ] Test X-Original-URL and X-Rewrite-URL headers
  - [ ] Test path normalization bypasses (.%252e, .%09.)
  - [ ] Test debug endpoint exposure (/debug/pprof/)
  - [ ] Add Kubernetes API response validation

- [ ] **CVE-2021-41773: Apache Path Traversal**
  - [ ] Implement detection function `checkApachePathTraversal()`
  - [ ] Test /etc/passwd access via path traversal
  - [ ] Test CGI-based RCE variant
  - [ ] Add unit tests

- [ ] **Linkerd SSRF via l5d-dtab**
  - [ ] Implement detection function `checkLinkerdSSRF()`
  - [ ] Add l5d-dtab header manipulation
  - [ ] Add OAST integration for callback detection
  - [ ] Add unit tests

- [ ] **Spring Boot Gateway Actuator**
  - [ ] Implement detection function `checkSpringGatewayActuator()`
  - [ ] Test /gateway/routes and /actuator/gateway/routes
  - [ ] Parse route configuration from responses
  - [ ] Add unit tests

## Priority 3: Medium-Impact Vulnerabilities (Week 3)

- [ ] **X-Forwarded-For 403 Bypass**
  - [ ] Implement detection function `checkXFFBypass()`
  - [ ] Test multiple X-Forwarded-For values
  - [ ] Test X-Real-IP header
  - [ ] Compare baseline 403 vs bypass 200 responses

- [ ] **Web Cache Poisoning**
  - [ ] Implement detection function `checkCachePoisoning()`
  - [ ] Test unkeyed headers (X-Forwarded-Host, X-Forwarded-Prefix, X-Forwarded-Proto)
  - [ ] Cache key detection and validation
  - [ ] Response reflection detection

- [ ] **CVE-2019-10092: Apache mod_proxy XSS**
  - [ ] Implement detection function `checkApacheModProxyXSS()`
  - [ ] Test backslash injection in paths
  - [ ] Detect "Proxy Error" page with malformed links
  - [ ] Add unit tests

- [ ] **Generic Nginx proxy_pass SSRF**
  - [ ] Implement detection function `checkNginxProxyPassSSRF()`
  - [ ] Test user-controlled URL parameters
  - [ ] Cloud metadata access testing (169.254.169.254)
  - [ ] Internal service discovery

## Integration & Testing (Week 4)

- [ ] **Performance Optimization**
  - [ ] Implement per-vulnerability rate limiting
  - [ ] Add timeout controls for slow checks
  - [ ] Optimize OAST polling intervals
  - [ ] Add check result caching

- [ ] **Comprehensive Testing**
  - [ ] Unit tests for all vulnerability checks (target: 80%+ coverage)
  - [ ] Integration tests with Docker-based vulnerable environments
  - [ ] Test with real Apache 2.4.49, Nginx 1.18, Kong 3.x
  - [ ] Security tests to ensure checks don't cause harm

- [ ] **Documentation**
  - [ ] Update README.md with proxy vulnerability features
  - [ ] Create examples/ directory with sample outputs
  - [ ] Document configuration options
  - [ ] Write usage guide for proxy vulnerability testing

- [ ] **Docker Test Environments**
  - [ ] Create docker-compose.yml with vulnerable services
  - [ ] Apache 2.4.49 container (CVE-2021-41773)
  - [ ] Nginx with off-by-slash misconfiguration
  - [ ] Kong Gateway with exposed admin API
  - [ ] Spring Boot Gateway application

## Output & Reporting Enhancements

- [ ] **JSON Output Enhancement**
  - [ ] Add `proxy_vulnerabilities` section to SecurityCheckResults
  - [ ] Include CVE references in output
  - [ ] Add evidence (request/response snippets)
  - [ ] Include remediation guidance

- [ ] **Terminal UI Updates**
  - [ ] Add proxy vulnerability section to verbose output
  - [ ] Color-coded severity indicators
  - [ ] Progress indicators for slow checks (OAST callbacks)

- [ ] **Metrics Collection**
  - [ ] Add Prometheus metrics for proxy vulnerability detections
  - [ ] Track check execution times
  - [ ] Count vulnerable proxies by type

## Configuration Implementation

- [ ] **Update config/default.yaml**
  - [ ] Add `proxy_vulnerabilities` section under `advanced_checks`
  - [ ] Individual enable/disable flags per vulnerability
  - [ ] Path/endpoint lists for each check
  - [ ] OAST integration settings

- [ ] **Configuration Validation**
  - [ ] Validate proxy vulnerability config on load
  - [ ] Provide helpful error messages for misconfigurations
  - [ ] Support hot-reload for proxy vulnerability settings

## Safety & Ethics

- [ ] **Rate Limiting & Throttling**
  - [ ] Aggressive rate limits for RCE checks (1 request per 10 seconds)
  - [ ] Configurable check delays
  - [ ] Respect robots.txt and security.txt

- [ ] **Fail-Safe Mechanisms**
  - [ ] Dry-run mode for testing without actual exploitation
  - [ ] Confirmation prompts for dangerous checks (RCE, destructive tests)
  - [ ] Audit logging for all vulnerability checks

- [ ] **Legal & Ethical Guidelines**
  - [ ] Document authorized use only
  - [ ] Add warning messages in CLI
  - [ ] Include responsible disclosure guidance in docs

## Future Enhancements (Backlog)

- [ ] Machine learning-based proxy fingerprinting
- [ ] Automated exploit chain discovery (path traversal → RCE)
- [ ] Integration with Metasploit modules
- [ ] Custom Nuclei template import
- [ ] Collaborative vulnerability database
- [ ] Real-time threat intelligence integration

---

## Resources

**Documentation:** `/Users/mgriffiths/Library/Mobile Documents/com~apple~CloudDocs/Projects/Code/ProxyHawk/docs/proxy-vulnerabilities-research.md`

**Key References:**
- Nuclei Templates: https://github.com/projectdiscovery/nuclei-templates
- Orange Tsai's Blog: https://blog.orange.tw/
- Apache Security: https://httpd.apache.org/security/vulnerabilities_24.html
- OWASP SSRF: https://owasp.org/www-community/attacks/Server_Side_Request_Forgery

---

**Total Estimated Time:** 4 weeks (160 hours)
**Priority 1 Completion Target:** Week 1
**Production Ready Target:** Week 4
