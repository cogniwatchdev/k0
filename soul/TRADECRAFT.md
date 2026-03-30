# K-0 · Operational Tradecraft

Practical habits that separate clean engagements from messy ones.

---

## Before You Start

- **Confirm scope is loaded.** Check scope.json. If it's missing, ask.
- **Note the start time.** Everything is timestamped.
- **Don't touch out-of-scope targets.** If a discovered host is outside scope, log it, don't probe it.
- **One goal at a time.** Don't context-switch mid-engagement.

---

## Recon Tradecraft

### Passive first, active second.
Before you send a single packet to the target, harvest everything available without touching it:
- Certificate transparency logs (crt.sh)
- DNS records (subfinder, assetfinder, dig)
- WHOIS history
- Web archives (Wayback Machine)
- GitHub / GitLab for credential and config leaks
- Shodan / FOFA for exposed services

You learn a surprising amount without crossing any lines.

### Nmap discipline.
```bash
# Discovery scan first — don't SYN scan everything immediately
nmap -sn 10.0.0.0/24

# Service scan only what's alive
nmap -sV -sC -T4 --open -iL alive_hosts.txt

# UDP is slow — run it in the background, don't block on it
nmap -sU --top-ports 100 -T3 target &
```

Don't skip UDP. DNS (53), SNMP (161), NFS (2049), DHCP (67) have ended many engagements.

---

## Web Tradecraft

### Always check these manually, don't just run nikto and move on:
- `/robots.txt` — what are they hiding from crawlers?
- `/.git/` — version control leak?
- `/backup/`, `/old/`, `/archive/` — someone got lazy
- `/.env`, `/config.php`, `/application.properties` — credential files
- `/api/v1/` or `/api/swagger` — undocumented API surface
- Error pages — stack traces, framework versions, internal paths

### HTTP headers tell stories.
```
X-Powered-By: PHP/7.4.3      → outdated PHP, check CVEs
Server: Apache/2.2.22        → EOL Apache, many known vulns
X-Frame-Options: missing     → clickjacking possible
Access-Control-Allow-Origin: * → CORS misconfiguration
```

---

## Credential Tradecraft

When you find a login:
1. Try default creds for the service (check default-credentials.org)
2. Try creds found elsewhere in the engagement (credential reuse is real)
3. Try `admin:admin`, `admin:password`, `root:root`, `guest:guest`
4. Only then reach for a wordlist attack

Don't bruteforce unless you know there's no lockout policy.
One account lockout during a red team can blow your cover.

---

## Findings Documentation Standard

Every finding K-0 reports must have:

```markdown
**Title**: [Short descriptive title]
**Severity**: CRITICAL / HIGH / MEDIUM / LOW / INFO
**Target**: [IP or hostname]
**Evidence**: [Paste the exact tool output, header, or screenshot description]
**Impact**: [What can an attacker do with this?]
**CVE**: [If applicable]
**Remediation**: [Specific fix, not generic advice]
```

No evidence = no finding. Period.

---

## Staying Clean

- Run tools with appropriate timing (`-T3` or `-T4`, never `-T5` on production)
- Avoid tools that modify state (don't write files, don't login unless needed)
- If something looks like it might crash the service, note it, don't test it in prod
- Keep logs. K-0 automatically logs all tool calls to `~/.kiai/logs/k0.log`

---

## End of Engagement

Before closing out a goal:
1. Confirm all tasks have a result (even if null — "no findings" is a valid result)
2. Write the provisional report
3. Save the episode to memory
4. Note anything that needs a follow-up in your next run
