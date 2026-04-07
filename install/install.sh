#!/bin/bash
# K-0 Agent — Full Installer for Kali Linux
# Usage: chmod +x install.sh && ./install.sh
#
# This script:
#   1. Installs system prerequisites (Go, Ollama) if missing
#   2. Builds K-0 from source
#   3. Pulls and configures the embedded AI model (~267MB)
#   4. Installs essential Kali pentest tools
#   5. Creates default config — zero manual setup needed
#
# After install, just type 'k0' to launch.

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
K0_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
K0_VERSION="0.4.0"
K0_BIN="$HOME/bin/k0"
K0_LAUNCHER="/usr/local/bin/k0"
K0_CFG="$HOME/.kiai/config.json"

RED='\033[0;31m'
GREEN='\033[0;32m'
PURPLE='\033[0;35m'
NC='\033[0m'

ok()   { echo -e "${GREEN}[✓]${NC} $1"; }
fail() { echo -e "${RED}[✗]${NC} $1"; }
info() { echo -e "${PURPLE}[…]${NC} $1"; }

echo ""
${NC}"echo -e "${PURPLE}╔═════════════════════════
echo -e "${PURPLE}║      K-0 Agent — Kali Installer       ║${NC}"
echo -e "${PURPLE}║          v${K0_VERSION} · LFM2.5-${NC}"350M            
{NC}"echo -e "${PURPLE}╚════════
echo ""

# ─── Step 1: System Prerequisites ───

info "Checking system prerequisites..."

# Check if running on Debian/Kali
if ! command -v apt &>/dev/null; then
    fail "This installer requires apt (Debian/Kali). Exiting."
    exit 1
fi
ok "Debian/Kali detected"

# Install Go if missing
if command -v go &>/dev/null; then
    ok "Go found: $(go version | awk '{print $3}')"
elif [ -x "$HOME/go-sdk/go/bin/go" ]; then
    export PATH="$HOME/go-sdk/go/bin:$PATH"
    ok "Go found at ~/go-sdk"
else
    info "Installing Go 1.22..."
    GO_TAR="go1.22.5.linux-amd64.tar.gz"
    wget -q "https://go.dev/dl/$GO_TAR" -O /tmp/$GO_TAR
    mkdir -p "$HOME/go-sdk"
    tar -xzf /tmp/$GO_TAR -C "$HOME/go-sdk"
    export PATH="$HOME/go-sdk/go/bin:$PATH"
    rm -f /tmp/$GO_TAR
    # Add to shell profile
    echo 'export PATH="$HOME/go-sdk/go/bin:$PATH"' >> "$HOME/.bashrc"
    ok "Go 1.22 installed"
fi

# Install Ollama if missing
if command -v ollama &>/dev/null; then
    ok "Ollama found"
else
    info "Installing Ollama..."
    curl -fsSL https://ollama.com/install.sh | sh 2>&1
    ok "Ollama installed"
fi

# ─── Step 2: K-0 Build ────

info "Building K-0..."
cd "$K0_ROOT"
go build -ldflags "-X main.Version=$K0_VERSION -X main.BuildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    -o ./k0 ./cmd/k0/ 2>&1
mkdir -p "$HOME/bin"
cp ./k0 "$K0_BIN"
ok "Built K-0 v$K0_VERSION"

# ─── Step 3: Launcher Install ─────────

cat > /tmp/k0-launcher << 'LAUNCHER'
#!/bin/bash
export COLORTERM=truecolor
export TERM=xterm-256color
exec "$HOME/bin/k0" "$@"
LAUNCHER

if [ -w /usr/local/bin ]; then
    cp /tmp/k0-launcher "$K0_LAUNCHER"
    chmod +x "$K0_LAUNCHER"
else
    sudo cp /tmp/k0-launcher "$K0_LAUNCHER"
    sudo chmod +x "$K0_LAUNCHER"
fi
rm -f /tmp/k0-launcher
ok "Launcher installed → k0"

# ─── Step 4: Pull & Create AI Model ──────────────

# Start Ollama if not running
if ! pgrep -x ollama &>/dev/null; then
    info "Starting Ollama..."
    ollama serve &>/dev/null &
    sleep 3
fi

if ollama list 2>/dev/null | grep -q "k0-pentest"; then
    ok "k0-pentest model exists"
else
    info "Pulling LFM2.5-350M base model (~267MB)..."
    ollama pull jewelzufo/LFM2.5-350M-GGUF:latest 2>&1
    info "Creating k0-pentest model..."
    ollama create k0-pentest -f "$K0_ROOT/model/Modelfile" 2>&1
    ok "k0-pentest model ready"
fi

# ─── Step 5: Install Kali Pentest Tools ───────

info "Checking Kali pentest tools..."

TOOLS_TO_INSTALL=""
KALI_TOOLS=(
    nmap nikto gobuster whatweb dirb wapiti fierce masscan
    sqlmap hydra medusa john hashcat
    enum4linux smbclient crackmapexec
    dnsrecon whois theharvester recon-ng
    searchsploit exploitdb
    aircrack-ng wifite
    metasploit-framework
)

for tool in "${KALI_TOOLS[@]}"; do
    pkg="$tool"
    # Map tool names to package names where they differ
    case "$tool" in
        searchsploit) pkg="exploitdb" ;;
        theharvester) pkg="theharvester" ;;
        exploitdb) continue ;; # handled by searchsploit
    esac
    if ! dpkg -l "$pkg" &>/dev/null 2>&1; then
        TOOLS_TO_INSTALL="$TOOLS_TO_INSTALL $pkg"
    fi
done

if [ -n "$TOOLS_TO_INSTALL" ]; then
    info "Installing missing tools:$TOOLS_TO_INSTALL"
    echo ""
    echo "  This will install Kali pentest tools (may require sudo)."
    echo "  Press Enter to continue or Ctrl+C to skip..."
    read -r
    sudo apt update -qq 2>/dev/null
    sudo apt install -y $TOOLS_TO_INSTALL 2>&1 || true
    ok "Kali tools installed"
else
    ok "All Kali pentest tools present"
fi

# ─── Step 6: Create Skills & Config ─────────────────────────────────────

mkdir -p "$HOME/.kiai/memory/episodes" "$HOME/.kiai/memory/knowledge" "$HOME/.kiai/memory/reports" "$HOME/.kiai/skills"

if [ ! -f "$K0_CFG" ]; then
    cat > "$K0_CFG" << CONFIG
{
  "ollama_addr": "http://127.0.0.1:11434",
  "model": "k0-pentest:latest",
  "memory_path": "$HOME/.kiai/memory",
  "semantic_memory": false,
  "summarize_every_mins": 20,
  "web_search_enabled": false,
  "telemetry": false,
  "theme": "kali-purple"
}
CONFIG
    ok "Config created at $K0_CFG"
else
    ok "Config exists"
fi

# Copy skills to user directory
if [ -d "$K0_ROOT/skills" ]; then
    cp -r "$K0_ROOT/skills/"* "$HOME/.kiai/skills/" 2>/dev/null || true
    ok "Skills loaded to ~/.kiai/skills/"
fi

Done # ─── ─────────────────────────────

echo ""
${NC}"echo -e "${PURPLE}╔═════════════════
echo -e "${     K-0 installed successfully!       ║${NC}"PURPLE}
echo -e "${PURPLE}║                                       ║${NC}"
echo -e "${PURPLE}║  Type 'k0' to launch the agent.       ║${NC}"
echo -e "${PURPLE}║                                       ║${NC}"
echo -e "${PURPLE}║  Model: LFM2.5-350M (~267MB)          ║${NC}"
echo -e "${  Config: ~/.kiai/config.json           ║${NC}"PURPLE}
echo -e "${PURPLE}║  Skills: ~/.kiai/skills/               ║${NC}"
{NC}"echo -e "${PURPLE}╚═════════════════════════════
