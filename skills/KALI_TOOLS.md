# K-0 Kali Linux Tool Reference
# This file auto-loads on startup and provides the AI model with
# comprehensive knowledge of available tools and their usage.

## Reconnaissance & Scanning

### nmap — Network Scanner
```
nmap -sV -sC -p- <target>          # Full port scan with version detection
nmap -sn <subnet>/24               # Host discovery (ping sweep)
nmap -A <target>                   # Aggressive scan (OS, version, scripts, traceroute)
nmap --script=vuln <target>        # Vulnerability scripts
nmap -sU -p 53,161,500 <target>    # UDP scan
nmap -Pn -sS <target>             # SYN stealth scan (no ping)
```

### masscan — High-Speed Port Scanner
```
masscan -p1-65535 <target> --rate=1000    # Full port scan at 1000 pps
masscan -p80,443 <subnet>/24 --rate=500  # Web ports on subnet
```

### whatweb — Web Fingerprinting
```
whatweb <url>                      # Identify web technologies
whatweb -a 3 <url>                 # Aggressive mode
whatweb --log-json=out.json <url>  # JSON output
```

### dnsrecon — DNS Enumeration
```
dnsrecon -d <domain>               # Standard enumeration
dnsrecon -d <domain> -t axfr       # Zone transfer attempt
dnsrecon -d <domain> -t brt        # Brute force subdomains
dnsrecon -d <domain> -t std        # Standard records
```

### fierce — DNS Reconnaissance
```
fierce --domain <domain>           # Subdomain discovery
fierce --domain <domain> --subdomains sub.txt  # Custom wordlist
```

### theHarvester — OSINT Email/Domain Harvesting
```
theHarvester -d <domain> -b all    # Harvest from all sources
theHarvester -d <domain> -b google,bing,linkedin
theHarvester -d <domain> -l 500    # Limit results
```

### recon-ng — OSINT Framework
```
recon-ng -w <workspace>            # Start with workspace
# Modules: recon/domains-hosts/*, recon/hosts-ports/*
```

### whois — Domain Registration Lookup
```
whois <domain>                     # Full registration info
```

---

## Web Application Testing

### nikto — Web Vulnerability Scanner
```
nikto -h <url>                     # Full web scan
nikto -h <url> -p 8080             # Custom port
nikto -h <url> -ssl                # HTTPS scan
nikto -h <url> -Tuning x           # Specific test types
nikto -h <url> -o report.html      # HTML report
```

### gobuster — Directory/File Brute-Force
```
gobuster dir -u <url> -w /usr/share/wordlists/dirb/common.txt
gobuster dir -u <url> -w /usr/share/wordlists/dirbuster/directory-list-2.3-medium.txt
gobuster dir -u <url> -x php,html,txt  # File extensions
gobuster dns -d <domain> -w subdomains.txt  # DNS brute-force
gobuster vhost -u <url> -w vhosts.txt       # Virtual host discovery
```

### dirb — URL Brute-Force
```
dirb <url>                         # Default wordlist
dirb <url> /usr/share/wordlists/dirb/big.txt
dirb <url> -a "Mozilla/5.0"       # Custom user-agent
```

wapiti ### Web Application Vulnerability Scanner 
```
wapiti -u <url>                    # Full scan
wapiti -u <url> -m sql,xss         # Specific modules
wapiti -u <url> --scope folder     # Limit scope
wapiti -u <url> -f html -o report  # HTML report
```

### sqlmap — SQL Injection Testing
```
sqlmap -u "<url>?id=1"             # Test URL parameter
sqlmap -u "<url>?id=1" --dbs       # Enumerate databases
sqlmap -u "<url>?id=1" --tables    # Enumerate tables
sqlmap -u "<url>?id=1" --dump      # Dump data
sqlmap -r request.txt              # Test from saved request
sqlmap -u "<url>" --forms          # Test forms automatically
sqlmap -u "<url>" --os-shell       # OS shell (if possible)
sqlmap -u "<url>" --risk=3 --level=5  # Maximum testing
```

---

## Exploitation

### Metasploit Framework (msfconsole)
```
msfconsole                         # Start Metasploit
search <keyword>                   # Search exploits
use exploit/<path>                 # Select exploit
show options                       # View required settings
set RHOSTS <target>                # Set target
set LHOST <attacker_ip>            # Set listener
set PAYLOAD <payload>              # Set payload
exploit / run                      # Execute exploit
```

#### Common Metasploit Modules
```
# Web Application Exploits
exploit/multi/http/apache_mod_cgi_bash_env_exec  # Shellshock
exploit/unix/webapp/wp_admin_shell_upload         # WordPress RCE
exploit/multi/http/tomcat_mgr_upload              # Tomcat Manager
exploit/multi/http/jenkins_script_console         # Jenkins RCE

# Network Exploits  
exploit/windows/smb/ms17_010_eternalblue          # EternalBlue
exploit/windows/smb/ms08_067_netapi               # Conficker
exploit/linux/samba/is_known_pipename              # SambaCry

# Service Exploits
exploit/unix/ftp/vsftpd_234_backdoor              # vsFTPd backdoor
exploit/multi/misc/java_rmi_server                 # Java RMI

# Post-Exploitation
post/multi/gather/firefox_creds                    # Browser creds
post/windows/gather/hashdump                       # Password hashes
post/linux/gather/enum_system                      # System enum
post/multi/manage/shell_to_meterpreter             # Upgrade shell

# Auxiliary Scanners
auxiliary/scanner/smb/smb_ms17_010                 # EternalBlue check
auxiliary/scanner/http/dir_scanner                  # Web directory scan
auxiliary/scanner/portscan/tcp                      # Port scanner
auxiliary/scanner/ssh/ssh_login                     # SSH brute force
```

### searchsploit — Exploit Database Search
```
searchsploit <software>            # Search for exploits
searchsploit <software> <version>  # Version-specific
searchsploit -m <exploit_id>       # Mirror/copy exploit
searchsploit -x <exploit_id>       # View exploit code
searchsploit --update              # Update database
```

---

## Brute Force & Password Attacks

### hydra — Network Login Brute-Force
```
hydra -l <user> -P <wordlist> <target> ssh
hydra -L users.txt -P pass.txt <target> ftp
hydra -l admin -P rockyou.txt <target> http-post-form "/login:user=^USER^&pass=^PASS^:Invalid"
hydra -l <user> -P <wordlist> <target> rdp
hydra -l <user> -P <wordlist> <target> mysql
hydra -l <user> -P <wordlist> <target> smb
```

### medusa — Parallel Network Login Brute-Force
```
medusa -h <target> -u <user> -P <wordlist> -M ssh
medusa -H hosts.txt -U users.txt -P pass.txt -M ftp
```

### john — Password Hash Cracker
```
john <hashfile>                    # Auto-detect and crack
john --wordlist=rockyou.txt <hashfile>
john --rules <hashfile>            # Apply mangling rules
john --show <hashfile>             # Show cracked passwords
john --format=raw-md5 <hashfile>   # Specify hash type
```

### hashcat — GPU Password Cracker
```
hashcat -m 0 <hashfile> <wordlist>     # MD5
hashcat -m 1000 <hashfile> <wordlist>  # NTLM
hashcat -m 1800 <hashfile> <wordlist>  # sha512crypt
hashcat -m 13100 <hashfile> <wordlist> # Kerberos TGS
hashcat -a 3 -m 0 <hashfile> ?a?a?a?a?a?a  # Brute force mask
```

---

## SMB / Active Directory

### enum4linux — SMB Enumeration
```
enum4linux -a <target>             # Full enumeration
enum4linux -U <target>             # Users
enum4linux -S <target>             # Shares
enum4linux -G <target>             # Groups
```

### smbclient — SMB Client
```
smbclient -L //<target>/ -N       # List shares (null session)
smbclient //<target>/<share>      # Connect to share
smbclient //<target>/<share> -U <user>
```

### crackmapexec — Network Pentest Swiss Army Knife
```
crackmapexec smb <target>          # SMB enumeration
crackmapexec smb <target> -u <user> -p <pass> --shares
crackmapexec smb <target> -u <user> -p <pass> --exec-method smbexec -x "whoami"
crackmapexec winrm <target> -u <user> -p <pass>
crackmapexec ssh <target> -u <user> -p <pass>
```

---

## Wireless

### aircrack-ng — WiFi Cracking Suite
```
airmon-ng start wlan0              # Enable monitor mode
airodump-ng wlan0mon               # Scan networks
airodump-ng -c <ch> --bssid <bssid> -w capture wlan0mon  # Capture
aireplay-ng -0 5 -a <bssid> wlan0mon    # Deauth attack
aircrack-ng -w <wordlist> capture.cap    # Crack WPA
```

### wifite — Automated WiFi Attacker
```
wifite                             # Automated scan and attack
wifite --kill                      # Kill conflicting processes
wifite -i wlan0mon                 # Specify interface
```

---

## Methodology

### PTES Phases
1. Pre-engagement → Define scope and rules of engagement
2. Intelligence Gathering → OSINT, DNS, network mapping
3. Threat Modeling → Identify attack surfaces
4. Vulnerability Analysis → Scan and enumerate
5. Exploitation → Attempt access
6. Post-Exploitation → Privilege escalation, lateral movement
7. Reporting → Document findings with severity ratings

### OWASP Top 10 Testing
1. A01 Broken Access Control → Test auth bypasses, IDOR
2. A02 Cryptographic Failures → Check SSL/TLS, weak ciphers
3. A03 Injection → SQL, NoSQL, OS command, LDAP
4. A04 Insecure Design → Logic flaws, missing controls
5. A05 Security Misconfiguration → Default creds, verbose errors
6. A06 Vulnerable Components → CVE search, outdated software
7. A07 Auth Failures → Brute force, session management
8. A08 Data Integrity Failures → Deserialization, CI/CD
9. A09 Logging Failures → Check monitoring gaps
10. A10 SSRF → Test internal network access

### Wordlists (Kali Default Paths)
```
/usr/share/wordlists/rockyou.txt
/usr/share/wordlists/dirb/common.txt
/usr/share/wordlists/dirb/big.txt
/usr/share/wordlists/dirbuster/directory-list-2.3-medium.txt
/usr/share/wordlists/seclists/  (if installed)
/usr/share/nmap/scripts/         (NSE scripts)
```
