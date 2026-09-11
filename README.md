# 🧠 local-ai-worker

[![Release](https://img.shields.io/badge/Release-v1.0-14b8a6.svg)](https://github.com/benzjeremy/local-ai-worker/releases)
[![License: GPL-3.0](https://img.shields.io/badge/License-GPL--3.0-blue.svg)](https://github.com/benzjeremy/local-ai-worker/blob/main/LICENSE)
[![Go: 1.22](https://img.shields.io/badge/Go-1.22-00ADD8.svg)](https://golang.org)
[![Security: Zero-Dummy](https://img.shields.io/badge/Security-Zero--Dummy--Standard-10b981.svg)](https://benzjeremy.github.io/local-ai-worker/)
[![Isolation: Localhost](https://img.shields.io/badge/Isolation-127.0.0.1%20Only-38bdf8.svg)](https://benzjeremy.github.io/)

> **Local-First AI Worker & Second Brain Assistant in Go 1.22**  
> Radikal lokaler KI-Mitarbeiter mit semantischer Obsidian-Vault RAG-Suche (Okapi BM25), Supervisor-Feedback-Lernschleife, CAD- & Bildschirmanalyse (Vision) sowie hardware-bewusster Zero-Cloud-Inferenz.

---

## ⚡ Warum local-ai-worker?

Unternehmen und Entwickler stehen oft vor dem Dilemma: Entweder werden vertrauliche interne Wissensdatenbanken (wie Obsidian Markdown Vaults) an US-Cloud-KIs gesendet – mit massiven Datenschutz- und Leak-Risiken – oder lokale Modelle bleiben isoliert und unwissend.

`local-ai-worker` löst dieses Problem:
1. **100% Local-First & Zero Cloud Leaks:** Alle Inferenz- und RAG-Schritte laufen autark auf der eigenen Hardware (`127.0.0.1:11434` Ollama). Kein einziges Byte verlässt das System.
2. **Obsidian Vault RAG:** Direkte Einbindung lokaler Markdown-Verzeichnisse. Automatischer Parser für Frontmatter, Headings, `#level-...` Knotenhierarchien und In-Memory Okapi BM25 Indexierung (< 5 ms Reaktionszeit).
3. **Lernfähigkeit durch Feedback-Loops:** Korrigiert der Benutzer oder Vorgesetzte eine ungenaue Antwort, speichert der Worker die Regel verschlüsselt im Vault. Zukünftige Abfragen werden automatisch mit Few-Shot Alignment-Regeln angereichert.
4. **Vision & CAD-Inspektion:** Direkte visuelle Analyse von technischen Zeichnungen, UI-Mockups und Screenshots über lokale Vision-Modelle (`llava:7b`).
5. **Vordefinierte Assistenzaufgaben:** Schnelle Generierung von Text-Zusammenfassungen, E-Mail-Entwürfen, CAD-Metadaten-Extraktion und automatisierten Vault-Graph-Audits.

---

## 🛡️ Echte Sicherheit aus dem Effeff (Zero-Dummy-Security)

Keine billige Scheinsicherheit, sondern echte Produktionshärtung ab Werk:
- **AES-256-GCM Verschlüsselung:** Alle gespeicherten Feedback-Historien, Notizen-Caches und Konfigurationen werden kryptografisch geschützt (`data/worker_vault.enc`).
- **PBKDF2 Schlüsselableitung:** Mindestens **100.000 Iterationen** mit SHA-256 und 32-Byte kryptografischem Salt.
- **Strict Localhost Isolation:** Der Dienst bindet ausschließlich an `127.0.0.1:<port>`. Keine Exposition im LAN oder Internet ohne Reverse Proxy.
- **Kryptografische Authentifizierung:** Jeder API-Aufruf erfordert ein 32-Byte CSPRNG Token (`X-Worker-Token`).
- **Anti-DNS-Rebinding:** Strikte Validierung des HTTP-`Host`-Headers. Fremde Domains werden sofort mit HTTP 403 abgewiesen.
- **Anti-CSRF:** Valider `Origin`-Filter verhindert Cross-Origin Attacken aus dem Browser.

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
local-ai-worker --vault-dir /pfad/zum/obsidian_vault
```

### Windows (x86_64):
Entpacke `local-ai-worker-v1.0-windows.zip` und starte `local-ai-worker.exe` in der Eingabeaufforderung.

---

## 🚀 CLI-Flags

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

## 🌐 REST API Endpunkte

Alle Endpunkte (außer `/health`) erfordern den Header `X-Worker-Token: <token>`.

| Methode | Pfad | Beschreibung |
|---|---|---|
| `GET` | `/health` | Health-Check, Versionsstatus und Anzahl indizierter Notizen |
| `POST` | `/ask` | Semantische RAG-Wissensabfrage auf den Obsidian-Vault |
| `POST` | `/feedback` | Korrektur einreichen (wird verschlüsselt gespeichert & gelernt) |
| `POST` | `/vision/analyze` | Bild- & CAD-Analyse via lokales Vision-Modell |
| `POST` | `/task/run` | Assistenzaufgabe ausführen (`summarize`, `email_draft`, `vault_audit`) |
| `POST` | `/knowledge/scan` | Manueller Re-Scan und Re-Index des Vaults |
| `GET` | `/knowledge/stats`| Statistiken über indizierte Notizen und Feedback-Historie |

---

## 👥 Mitwirkende & Credits

- **Jeremy Benz** ([@benzjeremy](https://github.com/benzjeremy) & [@jbenz1706](https://github.com/jbenz1706)) – Projektgründer & Lead Developer
- **AI-Assistenten (Pair Programming):** Google Antigravity & Claude Code
- © 2026 Jeremy Benz

---

## 📄 Lizenz

Dieses Projekt steht unter der **GNU General Public License, Version 3 (GPL-3.0)**.  
Weitere Informationen: [Offizielle Lizenz (GPL-3.0)](https://github.com/benzjeremy/local-ai-worker/blob/main/LICENSE)
