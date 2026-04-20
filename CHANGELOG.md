# K-0 Changelog

All notable changes to the K-0 project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [0.4.0] — 2026-04-19

### Added

- **LFM2.5-350M model migration** — swapped from Qwen3 to Liquid AI's LFM2.5-350M (267MB), purpose-built for tool calling, 300+ tok/s on CPU
- **Embedded k0-pentest model** — auto-installed via Ollama, zero user configuration required
- **Kali tools skill file** — `skills/KALI_TOOLS.md` with 30+ tools, Metasploit module references, PTES methodology, OWASP Top 10 procedures
- **Full Kali installer** — `install/install.sh` automates Go, Ollama, model pull, tool installs, config generation
- **k0-run execution harness** — headless goal runner (`cmd/k0-run`) that executes plan → tools → findings → report → memory save
- **k0-e2e orchestrator test** — `cmd/k0-e2e` for automated end-to-end testing of the planning pipeline
- **k0-headless** — `cmd/k0-headless` for non-interactive daemon-style operation
- **Memory store read API** — `ListEpisodes()`, `ListKnowledge()`, `ListReports()` with limit params, newest-first ordering
- **TUI Memory tab** — real data loading from `~/.kiai/memory/`. Episodes show date + outcome + tags + goal. Knowledge shows date + category + summary. Reports show filenames with location hint. Scroll support (j/k keys)
- **TUI Findings panel** — dedicated panel for real-time vulnerability findings with severity color coding
- **Live task progress feed** — TUI chat panel shows per-tool execution status, stdout, findings, and errors as they happen
- **Cyberpunk theme** — Kali-inspired purple/dark theme with KaliPurple accent, ToolLine dim style, SectionTitle headers, Divider rules
- **Streaming LLM responses** — token-by-token streaming from k0-pentest model to TUI
- **Scope enforcement** — hard refuse at orchestrator level for goals outside defined scope
- **Tool verification** — checks if each tool is installed before execution, prompts for auto-install
- **Per-tool timeouts** — each tool execution has its own timeout to prevent hung scans

### Changed

- **Template matching keywords expanded** — added "kerberos", "spn", "ldap enum" to SMB/AD template; added "web assess", "security posture", "web app test", "web pentest" to web vulnerability template
- **Nmap speed optimization** — all nmap commands now use `--top-ports 1000 --max-rtt-timeout 500ms`; removed `-O` (OS detection) from recon template for ~3-5x speed improvement on slow networks
- **Orchestrator persistence** — episodes, knowledge, entities, and reports are saved in correct order before TaskDoneMsg
- **Status messages** — added "Saving episode...", "Extracting knowledge...", "Memory persistence complete" messages during post-goal save
- **Config structure** — `MemoryPath` field (not MemoryDir); scope fields reorganized

### Fixed

- **types.Finding bug** — `Tool` field now correctly populated from task tool name in executor
- **Post-goal ordering** — persistence now completes before TaskDoneMsg returns
- **Template matching** — "subdomains" and "certificate transparency" now correctly match DNS enum template instead of generic recon

### Tests

- **21 agent tests** — orchestrator, goal parsing, scope enforcement, LLM integration (mock + live), template matching
- **5 LLM client tests** — Ollama connection, request/response, error handling, streaming
- **10 skills loader tests** — skill manifest parsing, category filtering, tool matching
- **6 memory store tests** — ListEpisodes, ListEpisodesLimit, ListKnowledge, ListKnowledgeEmpty, ListReports, ListEpisodesEmpty

---

## [0.3.1] — 2026-04-15

### Added
- Initial TUI with Bubbletea + Lipgloss
- Basic orchestrator with template matching
- Config management (`~/.kiai/config.json`)
- Qwen3 model support (later replaced by LFM2.5)

### Changed
- Project renamed from earlier prototype to K-0

---

## [0.2.0] — 2026-04-08

### Added
- First working prototype
- Goal input → plan generation
- nmap-based reconnaissance

---

[0.4.0]: https://github.com/cogniwatchdev/k0/releases/tag/v0.4.0
[0.3.1]: https://github.com/cogniwatchdev/k0/releases/tag/v0.3.1
[0.2.0]: https://github.com/cogniwatchdev/k0/releases/tag/v0.2.0