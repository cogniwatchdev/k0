<p align="center">
  <img src="docs/k0-banner.png" alt="K-0 TUI" width="700"/>
</p>

<h1 align="center">K-0</h1>

<p align="center">
  <strong>AI-powered offensive security agent for Kali Linux</strong>
</p>

<p align="center">
  <a href="#features">Features</a> •
  <a href="#install">Install</a> •
  <a href="#usage">Usage</a> •
  <a href="#architecture">Architecture</a> •
  <a href="#license">License</a>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/version-0.3.1-8b5cf6?style=flat-square" alt="Version"/>
  <img src="https://img.shields.io/badge/go-1.22+-00ADD8?style=flat-square&logo=go&logoColor=white" alt="Go"/>
  <img src="https://img.shields.io/badge/platform-Kali%20Linux-557C94?style=flat-square&logo=kalilinux&logoColor=white" alt="Kali"/>
  <img src="https://img.shields.io/badge/license-MIT-green?style=flat-square" alt="License"/>
  <img src="https://img.shields.io/badge/AI-embedded-FF6B6B?style=flat-square" alt="Embedded AI"/>
</p>

---

K-0 is a **self-contained, offline-first security agent** for Kali Linux. It ships with its own AI model — no API keys, no cloud services, no configuration. Install it, type `k0`, and start hacking.

The embedded **k0-pentest** model (built on [xploiter/pentester](https://ollama.com/xploiter/pentester)) is purpose-trained for offensive security. It understands pentest methodology, knows Kali tools, and generates structured attack plans from plain English goals. Everything runs locally on your machine.

<p align="center">
  <img src="docs/k0-demo.png" alt="K-0 in action" width="700"/>
</p>

## Features

### 🧠 Embedded AI — Zero Configuration
K-0 ships with everything it needs. The installer automatically:
- Pulls the `xploiter/pentester` base model
- Creates the `k0-pentest` tool-calling wrapper via the bundled `Modelfile`
- Generates a default config at `~/.kiai/config.json`
- Starts Ollama if it isn't running

**You never touch Ollama directly.** No API keys. No environment variables. Just `k0`.

### 🎯 Natural Language → Pentest Plan
Type your objective in plain English. K-0 generates a multi-phase attack plan:

```
goal: full recon on 192.168.1.0/24
```

K-0 responds:
```
Phase 1: Network Discovery
  └─ nmap -sn 192.168.1.0/24

Phase 2: Service Enumeration
  └─ nmap -sV -sC -p- <live_hosts>

Phase 3: Vulnerability Scan
  └─ nmap --script=vuln <targets>

Approve? [y/n]
```

### ⚡ Instant Template Matching
Common pentest patterns (web scan, recon, DNS, brute-force) are matched instantly — **zero LLM latency**. Only novel or complex goals hit the AI model.

### 🔒 Human-in-the-Loop
**Nothing runs without your explicit approval.** Every plan shows exact commands, risk level, tool availability, and scope boundaries before you confirm.

### 🛡️ Scope Enforcement
Define your engagement scope. K-0 hard-refuses anything outside it — at the orchestrator level, not as a suggestion.

### 🔧 30+ Kali Tools
Native support for the essential toolkit:

| Category | Tools |
|---|---|
| **Recon** | nmap, masscan, dnsrecon, whois, dig, fierce |
| **Web** | nikto, gobuster, whatweb, wapiti, dirb |
| **Exploitation** | searchsploit, msfconsole, sqlmap |
| **Brute Force** | hydra, medusa, john, hashcat |
| **OSINT** | theHarvester, recon-ng, maltego |
| **Wireless** | aircrack-ng, wifite |
| **Post-Exploit** | enum4linux, smbclient, crackmapexec |

K-0 verifies tool availability before planning and can request permission to install missing packages.

### 🖥️ Premium TUI
Built with [Bubbletea](https://github.com/charmbracelet/bubbletea) + [Lipgloss](https://github.com/charmbracelet/lipgloss):
- Multi-panel layout (Chat / Memory / Settings)
- Kali-inspired purple/dark theme
- Animated thinking indicator
- Keyboard-driven (Tab, Ctrl+L, Enter)
- Responsive layout

---

## Install

### Prerequisites
- **Kali Linux** (or any Debian-based distro with pentest tools installed)
- **Go 1.22+**
- **Ollama** installed (the installer handles everything else)

### One Command Install

```bash
git clone https://github.com/cogniwatchdev/k0.git
cd k0
chmod +x install/install.sh
./install/install.sh
```

That's it. The installer:
1. Builds the Go binary
2. Installs the `k0` launcher to `/usr/local/bin/`
3. Starts Ollama if needed
4. Pulls and creates the `k0-pentest` model automatically
5. Generates default config at `~/.kiai/config.json`

### Run

```bash
k0
```

No setup wizards. No API keys. Just type your goal.

---

## Usage

### Basic Flow

```
1. Launch          →  k0
2. Set your goal   →  goal: web scan example.com
3. Review the plan →  K-0 shows phases, tools, risk level
4. Confirm         →  y / n
5. Watch results   →  Real-time tool output in the chat panel
```

### Keyboard Shortcuts

| Key | Action |
|---|---|
| `Tab` | Switch panels (Chat / Memory / Settings) |
| `Enter` | Submit goal or confirm |
| `Ctrl+L` | Clear chat |
| `i` | Install missing tool (when prompted) |
| `y` / `n` | Confirm or reject plan |
| `q` / `Ctrl+C` | Quit |

### Example Goals

```
goal: full recon on 10.0.0.0/24
goal: web scan and directory brute-force on target.com
goal: DNS enumeration for example.org
goal: check for default credentials on 192.168.1.1
goal: OWASP Top 10 assessment of webapp.local
goal: wireless network discovery
```

---

## Architecture

```
┌─────────────────────────────────────────────────────┐
│                    K-0 TUI                           │
│              (Go / Bubbletea)                        │
├──────────┬──────────┬───────────┬───────────────────┤
│  Chat    │  Memory  │ Settings  │  Status Bar       │
│  Panel   │  Panel   │  Panel    │  ● READY v0.3.1   │
├──────────┴──────────┴───────────┴───────────────────┤
│                                                      │
│  Orchestrator                                        │
│  ├── Template Matcher (instant plans)                │
│  ├── LLM Planner (novel goals)                       │
│  ├── Scope Enforcer                                  │
│  └── Tool Executor (per-tool timeouts)               │
│                                                      │
├──────────────────────────────────────────────────────┤
│  Embedded k0-pentest model (via Ollama)              │
│  └── xploiter/pentester + tool-calling Modelfile     │
│     Auto-managed · No user configuration             │
└──────────────────────────────────────────────────────┘
```

### Key Design Decisions

- **Self-contained** — The AI model is embedded. The installer pulls it, wraps it with tool-calling support, and configures it automatically. Users never interact with Ollama.
- **Offline-first** — No cloud APIs, no telemetry, no data leaves the machine. Your scans and findings stay local.
- **Template matching first** — 6 common pentest patterns are matched instantly. Only novel objectives hit the AI model.
- **Per-tool timeouts** — Each tool execution has its own timeout to prevent hung scans.
- **Scope enforcement** — Hard boundaries at the orchestrator level. The AI cannot bypass scope restrictions.
- **Soul files** — Persona, methodology, and tradecraft are embedded as markdown files (`soul/`) that shape the AI's reasoning.

---

## Soul Files

K-0's thinking is shaped by markdown knowledge files in the `soul/` directory:

| File | Purpose |
|---|---|
| `PERSONA.md` | Agent identity and communication style |
| `MINDSET.md` | Red team methodology and thinking patterns |
| `TOOLS.md` | Tool selection heuristics and preferences |
| `TRADECRAFT.md` | Operational security guidance |
| `OWASP.md` | OWASP Top 10 testing methodology |
| `OSINT.md` | Open-source intelligence techniques |
| `METASPLOIT.md` | Metasploit Framework usage patterns |
| `REPORTING.md` | Report format and finding classification |

---

## Configuration

K-0 works out of the box. For power users, the config lives at `~/.kiai/config.json`:

```json
{
  "ollama_addr": "http://127.0.0.1:11434",
  "model": "k0-pentest:latest",
  "memory_path": "~/.kiai/memory",
  "semantic_memory": false,
  "web_search_enabled": false,
  "telemetry": false,
  "theme": "kali-purple"
}
```

Most users will never need to edit this. It's there if you want to point K-0 at a remote Ollama instance or change the theme.

---

## Roadmap

- [x] Multi-panel TUI with Bubbletea
- [x] Embedded k0-pentest model (auto-install)
- [x] Template matching for instant plans
- [x] Tool verification & auto-install
- [x] Scope enforcement
- [x] Animated thinking indicator
- [x] 30+ Kali tool integrations
- [ ] Streaming LLM responses
- [ ] PDF/HTML report export
- [ ] Plugin system for custom tools
- [ ] Multi-target campaign mode

---

## Contributing

```bash
git clone https://github.com/cogniwatchdev/k0.git
cd k0
go build -o k0 ./cmd/k0/
./k0
```

---

## Acknowledgements

- [xploiter/pentester](https://ollama.com/xploiter/pentester) — The base pentest model
- [Bubbletea](https://github.com/charmbracelet/bubbletea) — Terminal UI framework
- [Lipgloss](https://github.com/charmbracelet/lipgloss) — Style definitions for terminal layouts
- [Ollama](https://ollama.com) — Local LLM runtime
- [Kali Linux](https://www.kali.org) — The penetration testing platform

---

## License

MIT — see [LICENSE](LICENSE) for details.

Built with 💜 by [CogniWatch](https://cogniwatch.dev)

---

<p align="center">
  <sub>K-0 is a security research tool. Always obtain proper authorization before testing systems you don't own.</sub>
</p>
