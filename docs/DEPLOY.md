# K-0 Deployment Guide

## Test Environment

**Kali Server SSH Access:**
```bash
ssh -p 2233 k0@192.168.68.120
# Password: 0k
```

**Local Development:**
```bash
cd /root/.hermes/work/k-0
```

---

## TUI Logo Cutoff Fix (v0.4.0)

### Problem
The K-0 TUI logo was being cut off at the top — only the bottom 3 lines of the 6-line Unicode box-drawing logo were visible.

### Root Cause
Two issues in `internal/tui/model.go`:

1. **Incorrect line count**: `logoLineCount()` returned `7` but the full Unicode logo is `8` lines (6 art + divider + tagline)
2. **Excessive safety margin**: `safetyMargin = 2` added 2 extra blank lines, but terminal emulators (iTerm2, Terminal.app) report window height **including the title bar**, so those lines pushed the logo off-screen

### Fix

**File: `internal/tui/model.go`**

```go
// BEFORE (broken):
func logoLineCount(height int) int {
    if height < 38 {
        return 2 // compact logo
    }
    return 7 // WRONG - should be 8
}

const safetyMargin = 2 // WRONG - pushes logo off-screen

// AFTER (fixed):
func logoLineCount(height int) int {
    if height < 38 {
        return 2 // compact logo
    }
    return 8 // full Unicode logo: 6 art + 1 separator + 1 tagline
}

const safetyMargin = 0 // Terminal chrome already eats vertical space
```

**File: `internal/tui/logo.go`**

Ensure the full Unicode box-drawing logo is used (not ASCII `#` characters):

```go
func renderFullLogo(width int) string {
    artLines := []string{
        `██╗  ██╗       ██████╗ `,
        `██║ ██╔╝      ██╔═████╗`,
        `█████╔╝  ████╗██║██╔██║`,
        `██╔═██╗  ╚═══╝████╔╝██║`,
        `██║  ██╗      ╚██████╔╝`,
        `╚═╝  ╚═╝       ╚═════╝ `,
    }
    // ... rest of rendering
}
```

### Vertical Math (Exact Line Count)

```
Logo (full)        = 8 lines
Status dot         = 1 line
Blank separator    = 1 line
Tabs               = 1 line
Content panel      = contentH + 2 (borders via PanelFocused.Height)
Blank separator    = 1 line
Hints bar          = 1 line
Input bar          = 2 lines (border + field)
─────────────────────────────────────
Total chrome       = 17 lines
Content height     = m.height - 17
```

For **compact logo** (terminal height < 38 rows):
```
Total chrome = 11 lines (2-line logo instead of 8)
Content height = m.height - 11
```

### Build & Deploy

**Local rebuild:**
```bash
cd /root/.hermes/work/k-0
go build -o bin/k0 ./cmd/k0/main.go
./bin/k0 version  # Should show v0.4.0-dev
```

**Deploy to Kali test server:**
```bash
# Method 1: Pipe via SSH (works around scp permission issues)
cat bin/k0 | sshpass -p '0k' ssh -p 2233 k0@192.168.68.120 "cat > ~/bin/k0-new && chmod +x ~/bin/k0-new"

# Method 2: Replace on server
ssh -p 2233 k0@192.168.68.120
mv ~/bin/k0 ~/bin/k0.old
mv ~/bin/k0-new ~/bin/k0
~/bin/k0 version
```

### Verification

After deploying, run `k0` and verify:

1. **Full 6-line logo visible** at top (not cut off)
2. **Version shows `v0.4.0-dev`** in status bar
3. **Unicode box-drawing characters** (██) not ASCII `#`
4. **Status bar shows**: `● READY | v0.4.0-dev | LOCAL`

Expected logo output:
```
                                                ██╗  ██╗       ██████╗ 
                                                ██║ ██╔╝      ██╔═████╗
                                                █████╔╝  ████╗██║██╔██║
                                                ██╔═██╗  ╚═══╝████╔╝██║
                                                ██║  ██╗      ╚██████╔╝
                                                ╚═╝  ╚═╝       ╚═════╝ 
                                            ────────────────────────────────
                                               [ OPENCLAW ARCHITECTURE ]
                                                ● READY | v0.4.0-dev | LOCAL
```

---

## Binary Locations

| Location | Purpose |
|----------|---------|
| `/root/.hermes/work/k-0/bin/k0` | Local development binary |
| `/home/k0/bin/k0` | Kali test server binary |
| `~/bin/k0` | User's PATH binary (launcher) |

---

## Common Issues

### Logo still cut off
1. Check you're running the **new binary** (`./bin/k0 version` should say `v0.4.0-dev`)
2. Verify terminal height ≥ 38 rows (triggers full logo mode)
3. Check for terminal window chrome (title bar) eating vertical space

### Version shows v0.3.1
You're running the **old binary**. The new one is at `bin/k0`, not `k0` in the project root.

### SCP fails with "dest open Failure"
The `~/bin` directory may have permission issues. Use pipe method:
```bash
cat bin/k0 | ssh user@host "cat > ~/bin/k0-new && chmod +x ~/bin/k0-new"
```

### K-0 generates a plan when I say "hi"
**FIXED in v0.4.0-dev** — Added casual conversation detection. K-0 now recognizes greetings, thanks, status questions, and yes/no responses without triggering the planning engine.

**Detected conversation types:**
- Greetings: "hi", "hello", "hey", "yo", "good morning"
- Farewells: "bye", "goodbye", "see you", "later"
- Thanks: "thanks", "thank you", "thx", "cheers"
- Status: "how are you", "what can you do", "help", "version"
- Responses: "yes", "no", "y", "n", "sure", "ok"

**Example:**
```
You: hi
K-0: Hey! 👋 I'm K-0, your offensive security agent. Type your pentest goal...

You: what can you do
K-0: I'm built for pentesting: recon, web scans, DNS enum, brute-force...
```

---

## Git Status (v0.4.0-dev)

Modified files for this fix:
- `internal/tui/model.go` — safetyMargin, logoLineCount
- `internal/tui/logo.go` — Unicode logo restored

Commit message:
```
fix(tui): logo cutoff - correct line count and remove safety margin

- logoLineCount() now returns 8 for full Unicode logo (was 7)
- safetyMargin set to 0 - terminal chrome already consumes vertical space
- Restored Unicode box-drawing logo (was accidentally changed to ASCII #)

Fixes issue where top 3-4 lines of 6-line logo were scrolled off-screen
on terminals with window chrome (iTerm2, Terminal.app).
```
