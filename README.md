<p align="center">
  <img src="docs/k0-banner.png" alt="K-0 TUI" width="700"/>
</p>

<h1 align="center">K-0</h1>

<p align="center">
  <strong>AI-powered offensive security agent for Kali Linux</strong>
</p>

<p align="center">
  <a href="#features">Features</a> •
  <a href="#quick-start">Quick Start</a> •
  <a href="#usage">Usage</a> •
  <a href="#architecture">Architecture</a> •
  <a href="#configuration">Configuration</a> •
  <a href="#license">License</a>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/version-0.3.1-8b5cf6?style=flat-square" alt="Version"/>
  <img src="https://img.shields.io/badge/go-1.22+-00ADD8?style=flat-square&logo=go&logoColor=white" alt="Go"/>
  <img src="https://img.shields.io/badge/platform-Kali%20Linux-557C94?style=flat-square&logo=kalilinux&logoColor=white" alt="Kali"/>
  <img src="https://img.shields.io/badge/license-MIT-green?style=flat-square" alt="License"/>
  <img src="https://img.shields.io/badge/LLM-Ollama-FF6B6B?style=flat-square" alt="Ollama"/>
</p>

---

K-0 is an **offline-first, keyboard-driven TUI agent** that turns natural language security goals into structured pentest plans — then executes them with your explicit approval. Built in Go with [Bubbletea](https://github.com/charmbracelet/bubbletea) and designed for Kali Linux.

Think of it as a **co-pilot for red teamers** — you describe what you want to test, K-0 generates the plan, you confirm, and it runs the tools.

<p align="center">
  <img src="docs/k0-demo.png" alt="K-0 in action" width="700"/>
</p>

## Features

### 🎯 Natural Language → Pentest Plan
Type your objective in plain English. K-0 generates a multi-phase attack plan using the right tools for the job.

```
goal: full recon on 192.168.1.0/24
```

K-0 responds with a structured plan:
```
Phase 1: Network Discovery
  └─ nmap -sn 192.168.1.0/24

Phase 2: Service Enumeration  
  └─ nmap -sV -sC -p- <live_hosts>

Phase 3: Vulnerability Scan
  └─ nmap --script=vuln <targets>
```

### ⚡ Instant Template Matching
Common pentest patterns (web scan, recon, DNS, brute-force) are matched instantly — **zero LLM latency**. Novel goals fall back to the AI planner.

### 🔒 Human-in-the-Loop
**No tool runs without your explicit `y`.** Every plan shows:
- Exact commands to be executed
- Risk assessment
- Tool availability check
- Scope boundaries

### 🛡️ Scope Enforcement
Define your target scope. K-0 refuses to execute anything outside it. Hard boundaries, not suggestions.

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

### 🧠 AI-Powered Analysis
Powered by local LLMs via [Ollama](https://ollama.com). Fully offline — your data never leaves your machine.

### 🖥️ Premium TUI
Built with [Bubbletea](https://github.com/charmbracelet/bubbletea) + [Lipgloss](https://github.com/charmbracelet/lipgloss):
- Multi-panel layout (Chat / Memory / Settings)
- Kali-inspired purple/dark theme
- Animated thinking indicator
- Keyboard-driven (Tab, Ctrl+L, Enter)
- Responsive layout

---

## Quick Start

### Prerequisites
- **Kali Linux** (or any Debian-based distro with pentest tools)
- **Go 1.22+**
- **Ollama** running locally or on your network

### Install

#### Option 1: One-liner
```bash
curl -fsSL https://raw.githubusercontent.com/cogniwatchdev/k0/main/install/install.sh | bash
```

#### Option 2: Build from source
```bash
git clone https://github.com/cogniwatchdev/k0.git
cd k0
go build -o k0 ./cmd/k0/
sudo mv k0 /usr/local/bin/
```

#### Option 3: Go install
```bash
go install github.com/cogniwatchdev/k0/cmd/k0@latest
```

### Configure Ollama

K-0 needs an Ollama instance with a tool-calling capable model:

```bash
# If Ollama is on the same machine
ollama pull llama3.1

# Or point K-0 to a remote Ollama instance
export K0_OLLAMA_HOST=http://192.168.0.100:11434
```

### Run

```bash
k0
```

That's it. Type `goal: <your objective>` and go.

---

## Usage

### Basic Flow

```
1. Launch K-0           →  k0
2. Set your goal        →  goal: web scan example.com
3. Review the plan      →  K-0 shows phases, tools, risk level
4. Confirm execution    →  y / n
5. Watch results        →  Real-time tool output in the chat panel
6. Get findings         →  Automated report generation
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
│  ├── LLM Planner (novel goals → Ollama)              │
│  ├── Scope Enforcer                                  │
│  └── Tool Executor (per-tool timeouts)               │
│                                                      │
├──────────────────────────────────────────────────────┤
│  LLM Client ──► Ollama (local / remote)              │
│  Memory Store ──► ~/.k0/memory/                      │
│  Report Writer ──► ~/.k0/reports/                    │
└──────────────────────────────────────────────────────┘
```

### Key Design Decisions

- **Template matching first** — 6 common pentest patterns are matched instantly without any LLM call. Only novel objectives hit the AI.
- **Per-tool timeouts** — Each tool has its own execution timeout to prevent hung scans.
- **Scope enforcement** — Hard boundaries at the orchestrator level. The AI cannot bypass scope restrictions.
- **Soul files** — Persona, methodology, and tradecraft are embedded as markdown files (`soul/`) that shape the AI's reasoning.

---

## Configuration

K-0 stores its config at `~/.k0/config.yaml`:

```yaml
ollama:
  host: http://localhost:11434
  model: llama3.1              # Any Ollama model with tool-calling
  timeout: 300s

scope:
  targets:
    - 192.168.1.0/24           # Allowed target ranges
  excluded:
    - 192.168.1.1              # Exclude gateway

agent:
  max_phases: 5
  require_confirmation: true   # Never runs without approval
  auto_install_tools: false    # Ask before installing missing tools
```

### Environment Variables

| Variable | Default | Description |
|---|---|---|
| `K0_OLLAMA_HOST` | `http://localhost:11434` | Ollama API endpoint |
| `K0_MODEL` | `llama3.1` | Default model |
| `K0_DATA_DIR` | `~/.k0` | Data directory for memory and reports |

---

## Soul Files

K-0's personality and methodology are defined by markdown files in the `soul/` directory:

| File | Purpose |
|---|---|
| `PERSONA.md` | Core agent identity and communication style |
| `MINDSET.md` | Red team methodology and thinking patterns |
| `TOOLS.md` | Tool selection heuristics and preferences |
| `TRADECRAFT.md` | Operational security and stealth guidance |
| `OWASP.md` | OWASP Top 10 testing methodology |
| `OSINT.md` | Open-source intelligence techniques |
| `METASPLOIT.md` | Metasploit Framework usage patterns |
| `REPORTING.md` | Report format and finding classification |

---

## Roadmap

- [x] Multi-panel TUI with Bubbletea
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

Contributions are welcome! Please read our contributing guidelines before submitting a PR.

```bash
# Clone and build
git clone https://github.com/cogniwatchdev/k0.git
cd k0
go build -o k0 ./cmd/k0/
./k0
```

---

## Acknowledgements

- [Bubbletea](https://github.com/charmbracelet/bubbletea) — Terminal UI framework
- [Lipgloss](https://github.com/charmbracelet/lipgloss) — Style definitions for terminal layouts
- [Ollama](https://ollama.com) — Local LLM inference
- [Kali Linux](https://www.kali.org) — The penetration testing platform

---

## License

MIT — see [LICENSE](LICENSE) for details.

Built with 💜 by [CogniWatch](https://cogniwatch.dev)

---

<p align="center">
  <sub>K-0 is a security research tool. Always obtain proper authorization before testing systems you don't own.</sub>
</p>
