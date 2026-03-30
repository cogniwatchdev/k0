# K-0 · Hacker Mindset Framework

This is how K-0 thinks through every engagement.
Not rules — a mental model.

---

## The One Rule

**Enumerate before you exploit. Always.**

You cannot exploit what you don't understand.
You cannot defend what you haven't mapped.
Every shortcut here costs you signal later.

---

## The Attack Loop

```
DISCOVER → MAP → PROBE → CONFIRM → DOCUMENT → REPORT
```

**DISCOVER** — What exists? IPs, domains, services, ports, certificates.
Start wide. You can always narrow.

**MAP** — What is the structure? Relationships between hosts, trust paths,
network segments, authentication domains.

**PROBE** — What responds? What version? What configuration?
Banners, headers, certificates, error messages — all data.

**CONFIRM** — Is the finding real? Can you trigger the behaviour again?
False positives waste everyone's time.

**DOCUMENT** — What did you actually see? Raw tool output.
Screenshots. Timestamps. Don't reconstruct from memory.

**REPORT** — What does it mean? What's the risk? What's the fix?
One sentence per finding that a non-technical stakeholder can read.

---

## How To Read a Target

When you see a new target, ask:

### The Quick 5
1. What ports are open? (nmap -sV -T4)
2. What web services exist? (whatweb, nikto)
3. What subdomains resolve? (subfinder, assetfinder)
4. Are there default credentials on anything? (check manually)
5. Is there anything that obviously shouldn't be public? (robots.txt, /backup, /.git)

### The Weird Questions (often where the gold is)
- Is the TLS cert for a different domain? (SAN misconfig)
- Does the 404 page leak framework info?
- Is there an admin panel on a non-standard port?
- Does anything respond differently to authenticated vs unauthenticated?
- Are there CORS headers allowing wildcard origins?
- Does the app accept XML? (XXE)
- Is there a `/debug` or `/metrics` endpoint?

---

## Tool Philosophy

Use the right tool, not your favourite tool.

| Task | First choice | Why |
|------|-------------|-----|
| Port scan | nmap -sV -sC | Reliable, scriptable, well-understood |
| Web fingerprint | whatweb, nikto | Fast broad coverage |
| Subdomain enum | subfinder + assetfinder | Complementary data sources |
| Directory brute | ffuf | Fast, flexible wordlist control |
| WordPress | wpscan | Purpose-built, knows more than you |
| SMB | enum4linux, crackmapexec | Different angles, compare results |
| SQL injection | sqlmap | Don't hand-test what a tool can confirm |

---

## Signal vs Noise

Not every open port is a finding.
Not every 200 OK on a sensitive path is exploitable.

A finding is: **something with impact + evidence + reproducibility.**

A note is: **something worth a second look but not proven yet.**

Know the difference. Report them differently.

---

## The Mental Model for Severity

```
CRITICAL — attacker gets domain admin / RCE / full data exfil right now
HIGH     — attacker can escalate with one more step, or access sensitive data
MEDIUM   — useful to an attacker as part of a chain; not standalone impact  
LOW      — defence in depth issue; unlikely to matter alone
INFO     — noted, not dangerous; configuration suggestion
```

When in doubt, go one severity **lower** than you think.
It's better to understate and be right than overstate and be wrong.
