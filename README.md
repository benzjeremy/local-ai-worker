# 🧠 local-ai-worker

[![Release](https://img.shields.io/badge/Release-v1.0-14b8a6.svg)](https://github.com/benzjeremy/local-ai-worker/releases)
[![License: GPL-3.0](https://img.shields.io/badge/License-GPL--3.0-blue.svg)](https://github.com/benzjeremy/local-ai-worker/blob/main/LICENSE)
[![Go: 1.22](https://img.shields.io/badge/Go-1.22-00ADD8.svg)](https://golang.org)
[![Security: Zero-Dummy](https://img.shields.io/badge/Security-Zero--Dummy--Standard-10b981.svg)](https://benzjeremy.github.io/local-ai-worker/)
[![Isolation: Localhost](https://img.shields.io/badge/Isolation-127.0.0.1%20Only-38bdf8.svg)](https://benzjeremy.github.io/)

> **Local-First AI Worker & Second Brain Assistant in Go 1.22**  
> Radically local AI worker featuring semantic Obsidian Vault RAG search (Okapi BM25), supervisor feedback learning loop, CAD & screen inspection (Vision), and hardware-aware zero-cloud inference.

---

## ⚡ Why local-ai-worker?

Enterprises and developers face a dilemma: either send confidential internal knowledge bases (like Obsidian Markdown Vaults) to US cloud providers—incurring significant privacy and leak risks—or leave local models isolated without contextual knowledge.

`local-ai-worker` solves this challenge:
1. **100% Local-First & Zero Cloud Leaks:** All inference and RAG indexing run self-contained on local hardware (`127.0.0.1:11434` Ollama). Not a single byte leaves the machine.
2. **Obsidian Vault RAG:** Direct integration with local Markdown folders. Automatic parser for frontmatter, headings, `#level-...` node hierarchies, and in-memory Okapi BM25 indexing (< 5 ms retrieval latency).
3. **Continuous Feedback Learning Loops:** When a user or supervisor submits a correction, the worker securely stores the alignment rule in an encrypted vault. Future requests are automatically augmented with few-shot guidance.
4. **Vision & CAD Inspection:** Direct visual analysis of technical schematics, UI mockups, and screenshots via local vision models (`llava:7b`).
5. **Predefined Assistance Tasks:** Rapid generation of summaries, email drafts, CAD metadata extraction, and automated vault graph audits.

---

## 🛡️ Zero-Dummy-Security (Production-Hardened by Default)

No superficial security, but genuine production hardening:
- **AES-256-GCM Encryption:** All stored feedback histories, note caches, and configurations are cryptographically protected (`data/worker_vault.enc`).
- **PBKDF2 Key Derivation:** At least **100,000 iterations** with SHA-256 and a 32-byte cryptographic salt.
- **Strict Localhost Isolation:** The service strictly binds to `127.0.0.1:<port>`. No exposure to LAN or public interfaces without a reverse proxy.
- **Cryptographic Token Authentication:** Every API request requires a 32-byte CSPRNG token (`X-Worker-Token`).
- **Anti-DNS-Rebinding:** Strict HTTP `Host` header validation rejects foreign hostnames with HTTP 403.
- **Anti-CSRF:** Valid `Origin` filter prevents cross-origin browser attacks.

---

## 📦 Multi-Platform Installation & Download

### Via Go Install:
```bash
go install github.com/benzjeremy/local-ai-worker@latest
```

### Linux (x86_64):
```bash
tar -xzf local-ai-worker-v1.0-linux.tar.gz
sudo cp local-ai-worker /usr/local/bin/
local-ai-worker --vault-dir /path/to/obsidian_vault
```

### Windows (x86_64):
Extract `local-ai-worker-v1.0-windows.zip` and run `local-ai-worker.exe` in your terminal.

---

## 🚀 CLI Flags

```text
Usage of local-ai-worker:
  --port int            Port to listen on (default 8081, binds to 127.0.0.1)
  --token string        32-byte security token (auto-generated if omitted)
  --vault-dir string    Path to local Obsidian Vault directory
  --ollama-url string   URL of local Ollama API (default "http://127.0.0.1:11434")
  --model string        Default LLM for reasoning and text (default "llama3.1:8b")
  --vision-model string Default vision model (default "llava:7b")
  --data-dir string     Directory for encrypted storage vault (default "data")
  --scan-interval dur   Interval for automatic vault rescan (default 1m0s)
  --version             Print version and exit
```

---

## 🌐 REST API Endpoints

All endpoints (except `/health`) require the header `X-Worker-Token: <token>`.

| Method | Path | Description |
|---|---|---|
| `GET` | `/health` | Health check, version status, and indexed note counts |
| `POST` | `/ask` | Semantic RAG knowledge query against local Obsidian Vault |
| `POST` | `/feedback` | Submit correction (encrypted, saved, and learned) |
| `POST` | `/vision/analyze` | Image and CAD analysis via local vision model |
| `POST` | `/task/run` | Execute assistance task (`summarize`, `email_draft`, `vault_audit`) |
| `POST` | `/knowledge/scan` | Manual re-scan and re-index of vault |
| `GET` | `/knowledge/stats`| Statistics on indexed notes and feedback history |

---

## 👥 Contributors & Credits

- **Jeremy Benz** ([@benzjeremy](https://github.com/benzjeremy) & [@jbenz1706](https://github.com/jbenz1706)) – Project Founder & Lead Developer
- **AI Assistants (Pair Programming):** Google Antigravity & Claude Code
- © 2026 Jeremy Benz

---

## 📄 License

This project is licensed under the **GNU General Public License, Version 3 (GPL-3.0)**.  
See [LICENSE](LICENSE) for details.
