#!/bin/bash
# K-0 launcher — just type 'k0' to start
# Install: chmod +x install.sh && ./install.sh

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
K0_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
GO_BIN="$HOME/go-sdk/go/bin"
K0_BIN="$HOME/bin/k0"
K0_LAUNCHER="/usr/local/bin/k0"

echo "╔═══════════════════════════════════════╗"
echo "║         K-0 Agent — Installer         ║"
echo "╚═══════════════════════════════════════╝"
echo ""

# Step 1: Ensure Go is available
if [ -x "$GO_BIN/go" ]; then
    export PATH="$GO_BIN:$PATH"
    echo "[✓] Go found at $GO_BIN"
elif command -v go &>/dev/null; then
    echo "[✓] Go found in PATH"
else
    echo "[✗] Go not found. Install Go 1.22+ first."
    exit 1
fi

# Step 2: Build K-0
echo "[…] Building K-0..."
cd "$K0_ROOT"
go build -ldflags "-X main.Version=0.3.0 -X main.BuildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    -o ./k0 ./cmd/k0/ 2>&1
mkdir -p "$HOME/bin"
cp ./k0 "$K0_BIN"
echo "[✓] Built and installed to $K0_BIN"

# Step 3: Create launcher script at /usr/local/bin/k0
# This is what makes 'k0' work from anywhere
cat > /tmp/k0-launcher << 'LAUNCHER'
#!/bin/bash
# K-0 Agent launcher — sets up terminal environment and runs K-0
export COLORTERM=truecolor
export TERM=xterm-256color
exec "$HOME/bin/k0" "$@"
LAUNCHER

if [ -w /usr/local/bin ]; then
    cp /tmp/k0-launcher "$K0_LAUNCHER"
    chmod +x "$K0_LAUNCHER"
    echo "[✓] Launcher installed at $K0_LAUNCHER"
else
    echo "[…] Need sudo to install launcher to /usr/local/bin/"
    sudo cp /tmp/k0-launcher "$K0_LAUNCHER"
    sudo chmod +x "$K0_LAUNCHER"
    echo "[✓] Launcher installed at $K0_LAUNCHER (via sudo)"
fi
rm -f /tmp/k0-launcher

# Step 4: Ensure Ollama is running
if pgrep -x ollama &>/dev/null; then
    echo "[✓] Ollama is running"
else
    echo "[…] Starting Ollama..."
    ollama serve &>/dev/null &
    sleep 2
    echo "[✓] Ollama started"
fi

# Step 5: Pull/create the K-0 pentest model
if ollama list 2>/dev/null | grep -q "k0-pentest"; then
    echo "[✓] k0-pentest model already exists"
else
    echo "[…] Pulling xploiter/pentester base model..."
    ollama pull xploiter/pentester:latest 2>&1
    echo "[…] Creating k0-pentest wrapper model..."
    ollama create k0-pentest -f "$K0_ROOT/model/Modelfile" 2>&1
    echo "[✓] k0-pentest model created"
fi

# Step 6: Create default config if needed
K0_CFG="$HOME/.kiai/config.json"
if [ ! -f "$K0_CFG" ]; then
    mkdir -p "$HOME/.kiai/memory/episodes" "$HOME/.kiai/memory/knowledge" "$HOME/.kiai/memory/reports"
    cat > "$K0_CFG" << CONFIG
{
  "ollama_addr": "http://127.0.0.1:11434",
  "model": "k0-pentest:latest",
  "memory_path": "$HOME/.kiai/memory",
  "semantic_memory": false,
  "summarize_every_mins": 20,
  "gateway_addr": "http://127.0.0.1:19876",
  "web_search_enabled": false,
  "telemetry": false,
  "theme": "kali-purple"
}
CONFIG
    echo "[✓] Config created at $K0_CFG"
else
    echo "[✓] Config exists at $K0_CFG"
fi

echo ""
echo "╔═══════════════════════════════════════╗"
echo "║     K-0 installed successfully!       ║"
echo "║                                       ║"
echo "║  Type 'k0' to launch the agent.       ║"
echo "║  Type 'k0 setup' for first-time       ║"
echo "║  configuration wizard.                ║"
echo "║  Type 'k0 version' to check version.  ║"
echo "╚═══════════════════════════════════════╝"
