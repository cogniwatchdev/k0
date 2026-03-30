# K-0 · OSINT & Reconnaissance Reference

OSINT is the phase that separates good operators from mediocre ones.
You learn more passively than most people learn actively.
Never touch a target before you've exhausted passive sources.

---

## The OSINT Hierarchy

```
Level 1 — Fully Passive (no contact with target infrastructure)
  └── DNS records, WHOIS, cert transparency, Shodan, social media,
      job postings, GitHub, cached pages, breach data

Level 2 — Semi-Passive (normal user traffic, target may see you)
  └── Visiting public web pages, Google dorking, LinkedIn scraping,
      subdomain enumeration via DNS resolvers

Level 3 — Active (target WILL see you — requires authorisation)
  └── Port scanning, banner grabbing, web crawling, authenticated enumeration
```

Always complete Level 1 → Level 2 before going active.

---

## Domain & DNS Intelligence

### WHOIS
```bash
whois example.com
whois 203.0.113.1          # Reverse WHOIS on IP

# Historical WHOIS (useful for tracking ownership changes)
# → viewdns.info/whois, domaintools.com
```

### DNS enumeration
```bash
# Standard records
dig example.com ANY
dig example.com A
dig example.com MX
dig example.com TXT        # SPF, DMARC, DKIM, verification tokens
dig example.com NS
dig example.com AXFR @ns1.example.com  # Zone transfer attempt (often fails)

# Reverse DNS
dig -x 203.0.113.1

# DNS brute force
dnsx -l subdomains.txt -a -cname -resp
dnsrecon -d example.com -t brt -D wordlist.txt

# DNSdumpster — visual DNS recon
# → dnsdumpster.com
```

**What to look for in TXT records:**
- `v=spf1` — valid sending hosts (reveals infrastructure)
- `_dmarc` — email security posture
- Verification tokens (Google, Facebook) — confirms domain ownership
- Internal hostnames leaking into SPF records

### ASN & IP range discovery
```bash
# Find all IP ranges belonging to an org
whois -h whois.radb.net -- '-i origin AS12345'

# BGP lookups
bgp.he.net              # Hurricane Electric BGP toolkit
# Search org name → find ASNs → find subnets

# Shodan org search
shodan search org:"Target Corp" --fields ip_str,port,hostnames
```

---

## Certificate Transparency

TLS certs are publicly logged. Find subdomains before they're discovered any other way.

```bash
# crt.sh query
curl -s "https://crt.sh/?q=%25.example.com&output=json" | \
  jq -r '.[].name_value' | sort -u | grep -v '\*'

# Certspotter
curl -s "https://api.certspotter.com/v1/issuances?domain=example.com&expand=dns_names" | \
  jq -r '.[].dns_names[]' | sort -u

# Tools that automate this
subfinder -d example.com    # includes crt.sh backend
amass enum -passive -d example.com
```

---

## Shodan

Shodan indexes internet-connected devices. It often knows more about a target than the target does.

```bash
# Install
pip3 install shodan
shodan init YOUR_API_KEY

# Search
shodan search org:"Target Corp"
shodan search hostname:example.com
shodan search "default password" country:AU port:80
shodan search apache country:US city:"New York"

# Host info
shodan host 203.0.113.1

# Download all results
shodan download results.json.gz 'org:"Target Corp"'
shodan parse results.json.gz --fields ip_str,port,hostnames

# Interesting filters
port:445 os:"Windows Server 2008"           # old SMB
port:3389 country:AU                         # RDP exposed
vuln:CVE-2021-44228                          # Log4Shell exposed hosts
http.title:"Dashboard"                       # exposed dashboards
http.html:"Index of /"                       # open directories
```

**Shodan Dorks:**
- `http.favicon.hash:` — identify specific products by favicon
- `ssl.cert.subject.CN:` — find all hosts with the same cert
- `net:` — search within CIDR range
- `before:/after:` — time-bounded searches

---

## Google Dorking

```bash
# Site enumeration
site:example.com -www

# File types
site:example.com filetype:pdf
site:example.com filetype:xlsx OR filetype:docx
site:example.com filetype:sql OR filetype:bak OR filetype:env

# Exposed panels
site:example.com inurl:admin
site:example.com intitle:"index of"
site:example.com inurl:login intitle:admin

# Config / credential files
site:example.com ext:xml OR ext:conf OR ext:cnf OR ext:config
site:example.com inurl:.env

# Error pages with stack traces
site:example.com "Parse error" "on line"
site:example.com "Warning:" "mysql_"

# Cached / historic content
cache:example.com

# Combine with github
site:github.com example.com password OR secret OR api_key
```

---

## GitHub OSINT

Developers accidentally commit credentials constantly.

```bash
# Tools
trufflehog github --org=TargetOrg --only-verified
gitleaks detect --source=/path/to/repo
gitdorker -q "org:TargetOrg password" -o results.txt

# Manual searches on github.com
"example.com" password
"example.com" api_key
"example.com" secret
"example.com" private_key
org:TargetOrg "AKIA"    # AWS access keys start with AKIA
```

**What to look for:**
- AWS access keys (AKIA...)
- Private keys (-----BEGIN RSA PRIVATE KEY-----)
- Database connection strings
- `.env` files committed by accident
- Config files with hardcoded credentials
- Internal IP addresses and hostnames

---

## Email & Identity OSINT

```bash
# Employee email discovery
theHarvester -d example.com -b google,linkedin,twitter -f results

# Email permutation
# If you know john.smith@example.com exists, try: jsmith, john, j.smith...
# Tools: linkedin2username, email-permutator

# Breach data
# haveibeenpwned.com API
# DeHashed (paid)
# Snusbase (paid)
# → Cross-reference leaked passwords with current login attempts

# LinkedIn (manual)
# Find employees → Roles → Tech stack → Org structure
# Job postings → "Required: experience with Kubernetes, AWS, Oracle DB"
#              → reveals their exact tech stack
```

---

## Web Archive & Cache

```bash
# Wayback Machine — find old content, deleted pages, historic configs
curl "https://web.archive.org/cdx/search/cdx?url=*.example.com&output=json&fl=original&collapse=urlkey" | \
  python3 -c "import sys,json; [print(i[0]) for i in json.load(sys.stdin)[1:]]"

# gau — fetch known URLs from Wayback, OTX, Common Crawl
gau example.com | grep -E "\.(php|asp|aspx|jsp|json|xml|conf|bak|sql|env)"

# waybackurls  
echo example.com | waybackurls | sort -u > wayback_urls.txt
```

**Gold in old URLs:** Old admin panels, removed API endpoints, backup files (.bak), debug pages.

---

## Infrastructure Mapping

### Reverse IP — find co-hosted domains
```bash
# Free options
curl "https://api.hackertarget.com/reverseiplookup/?q=203.0.113.1"

# viewdns.info/reverseip
# Useful for shared hosting — other sites on same IP may have weaker security
```

### Port/Banner history
```bash
# Censys (create free account)
censys search "example.com" --index-type hosts
censys view hosts 203.0.113.1

# FOFA (alternative to Shodan)
fofa query 'domain="example.com"'
```

### Cloud asset discovery
```bash
# S3 bucket enumeration
aws s3 ls s3://target-backups --no-sign-request
s3scanner scan --bucket-file bucket_names.txt

# Common bucket names to try
target, target-backup, target-dev, target-staging, target-data, target-files,
target-assets, target-uploads, target-images, target-logs

# Azure blob
# https://targetname.blob.core.windows.net/container/

# GCP buckets
# https://storage.googleapis.com/targetname-bucket/
```

---

## OSINT Tools Quick Reference

| Tool | Purpose | Install |
|------|---------|---------|
| theHarvester | Multi-source email/subdomain | apt install theharvester |
| maltego | Visual link analysis | included in Kali |
| recon-ng | OSINT framework | apt install recon-ng |
| spiderfoot | Automated OSINT | apt install spiderfoot |
| sherlock | Username across platforms | pip3 install sherlock-project |
| holehe | Email → account check | pip3 install holehe |
| maigret | Username OSINT | pip3 install maigret |
| exiftool | File metadata | apt install libimage-exiftool-perl |
| metagoofil | Document metadata harvesting | apt install metagoofil |
| photon | Fast web crawler/OSINT | pip3 install photon |

---

## OSINT Report Checklist

Before finishing passive recon, confirm you have:

- [ ] All known subdomains (20+ sources)
- [ ] All IP ranges / ASNs attributed to target
- [ ] Employee list (LinkedIn + theHarvester)
- [ ] Email format confirmed
- [ ] Tech stack from job postings / headers / HTTP responses
- [ ] Any breach data / leaked credentials
- [ ] GitHub / GitLab repos reviewed
- [ ] Shodan inventory of exposed services
- [ ] Historic URLs from Wayback Machine
- [ ] All TLS certs enumerated
- [ ] Cloud storage buckets checked
