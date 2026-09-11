package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/benzjeremy/local-ai-worker/api"
	"github.com/benzjeremy/local-ai-worker/client"
	"github.com/benzjeremy/local-ai-worker/feedback"
	"github.com/benzjeremy/local-ai-worker/knowledge"
	"github.com/benzjeremy/local-ai-worker/storage"
	"github.com/benzjeremy/local-ai-worker/tasks"
	"github.com/benzjeremy/local-ai-worker/vision"
)

const (
	Version = "v1.0"
	Banner  = `
  ███████╗███████╗ ██████╗ ██████╗ ███╗   ██╗██████╗     ██████╗ ██████╗  █████╗ ██╗███╗   ██╗
  ██╔════╝██╔════╝██╔════╝██╔═══██╗████╗  ██║██╔══██╗    ██╔══██╗██╔══██╗██╔══██╗██║████╗  ██║
  ███████╗█████╗  ██║     ██║   ██║██╔██╗ ██║██║  ██║    ██████╔╝██████╔╝███████║██║██╔██╗ ██║
  ╚════██║██╔══╝  ██║     ██║   ██║██║╚██╗██║██║  ██║    ██╔══██╗██╔══██╗██╔══██║██║██║╚██╗██║
  ███████║███████╗╚██████╗╚██████╔╝██║ ╚████║██████╔╝    ██████╔╝██║  ██║██║  ██║██║██║ ╚████║
  ╚══════╝╚══════╝ ╚═════╝ ╚═════╝ ╚═╝  ╚═══╝╚═════╝     ╚═════╝ ╚═╝  ╚═╝╚═╝  ╚═╝╚═╝╚═╝  ╚═══╝
               LOCAL-FIRST AI WORKER & OBSIDIAN RAG AGENT (Go 1.22)
`
)

func main() {
	portFlag := flag.Int("port", 8081, "Port to listen on (binds exclusively to 127.0.0.1)")
	tokenFlag := flag.String("token", "", "32-byte security token (auto-generated if omitted)")
	vaultDirFlag := flag.String("vault-dir", "/home/benzj/Projekte/benzjeremy.github.io/obsidian_vault", "Path to local Obsidian Vault directory")
	ollamaURLFlag := flag.String("ollama-url", "http://127.0.0.1:11434", "URL of local Ollama API")
	modelFlag := flag.String("model", "llama3.1:8b", "Default LLM for reasoning and text tasks")
	visionModelFlag := flag.String("vision-model", "llava:7b", "Default vision model for image inspection")
	dataDirFlag := flag.String("data-dir", "data", "Directory for encrypted storage vault")
	scanIntervalFlag := flag.Duration("scan-interval", 60*time.Second, "Interval for automatic vault rescan")
	versionFlag := flag.Bool("version", false, "Print version and exit")

	flag.Parse()

	if *versionFlag {
		fmt.Printf("local-ai-worker %s (Jeremy Benz Zero-Dummy-Security)\n", Version)
		os.Exit(0)
	}

	fmt.Print(Banner)
	fmt.Println("================================================================================")
	fmt.Printf(" [SYSTEM] Version:         %s\n", Version)
	fmt.Printf(" [SYSTEM] Host Bind:       127.0.0.1:%d (Strict Localhost Isolation)\n", *portFlag)
	fmt.Printf(" [VAULT]  Target Vault:    %s\n", *vaultDirFlag)
	fmt.Printf(" [AI]     Ollama Endpoint: %s\n", *ollamaURLFlag)
	fmt.Printf(" [AI]     Text Model:      %s\n", *modelFlag)
	fmt.Printf(" [AI]     Vision Model:    %s\n", *visionModelFlag)
	fmt.Println("================================================================================")

	// 1. Initialize encrypted vault storage
	if err := os.MkdirAll(*dataDirFlag, 0700); err != nil {
		log.Fatalf("[-] Failed to create data directory: %v", err)
	}
	vaultPath := filepath.Join(*dataDirFlag, "worker_vault.enc")
	vaultPassphrase := os.Getenv("WORKER_VAULT_KEY")
	if vaultPassphrase == "" {
		vaultPassphrase = "DefaultJeremyBenzLocalWorkerEncryptedVaultKey2026!"
	}

	encVault, err := storage.OpenVault(vaultPath, vaultPassphrase)
	if err != nil {
		log.Fatalf("[-] Failed to initialize encrypted vault: %v", err)
	}
	fmt.Printf(" [SEC]    Encrypted Vault: %s (AES-256-GCM + PBKDF2 100k rounds)\n", vaultPath)

	// 2. Initialize Knowledge Base & Scanner
	scanner := knowledge.NewScanner(*vaultDirFlag)
	indexer := knowledge.NewIndexer()

	fmt.Println(" [*] Scanning Obsidian Vault Markdown files...")
	docs, err := scanner.Scan()
	if err != nil {
		fmt.Printf(" [!] Notice: Initial vault scan returned: %v (will retry in background)\n", err)
	} else {
		indexer.Build(docs)
		fmt.Printf(" [✓] Vault Indexed:      %d notes successfully loaded into memory\n", len(docs))
	}

	// 3. Initialize AI Services & Engines
	ollamaCli := client.NewOllamaClient(*ollamaURLFlag)
	fbEngine := feedback.NewEngine(encVault)
	visAnalyzer := vision.NewAnalyzer(ollamaCli, *visionModelFlag)
	taskExec := tasks.NewExecutor(ollamaCli, indexer, fbEngine, *modelFlag)

	// Check Ollama connection
	ctxPing, cancelPing := context.WithTimeout(context.Background(), 3*time.Second)
	if ollamaCli.Ping(ctxPing) {
		fmt.Println(" [✓] Ollama Server:      ONLINE (localhost:11434)")
	} else {
		fmt.Println(" [!] Ollama Server:      OFFLINE or UNREACHABLE (local queries will fail until Ollama is up)")
	}
	cancelPing()

	// 4. Initialize Secure REST API Server
	cfg := api.ServerConfig{
		Port:         *portFlag,
		Token:        *tokenFlag,
		VaultDir:     *vaultDirFlag,
		DefaultModel: *modelFlag,
		Version:      Version,
	}

	server, err := api.NewServer(cfg, scanner, indexer, fbEngine, visAnalyzer, taskExec, ollamaCli)
	if err != nil {
		log.Fatalf("[-] Failed to configure API server: %v", err)
	}

	if err := server.Start(); err != nil {
		log.Fatalf("[-] Failed to start HTTP server: %v", err)
	}

	fmt.Println("--------------------------------------------------------------------------------")
	fmt.Printf(" [SEC] API Security Token: %s\n", server.Token())
	fmt.Println("       Pass as Header:   'X-Worker-Token: <token>'")
	fmt.Println("--------------------------------------------------------------------------------")
	fmt.Println(" [API] Available Endpoints:")
	fmt.Println("       GET  /health          - Service status and vault telemetry")
	fmt.Println("       POST /ask             - Semantic RAG query against Obsidian Vault")
	fmt.Println("       POST /feedback        - Submit supervisor corrections for learning")
	fmt.Println("       POST /vision/analyze  - Image & CAD inspection via vision LLM")
	fmt.Println("       POST /task/run        - Run predefined task (summarize, audit, etc.)")
	fmt.Println("       POST /knowledge/scan  - Force vault rescan & index update")
	fmt.Println("       GET  /knowledge/stats - Vault index statistics")
	fmt.Println("================================================================================")
	fmt.Println(" [✓] Local AI Worker active and listening.")

	// Background Rescan Loop
	ticker := time.NewTicker(*scanIntervalFlag)
	stopRescan := make(chan struct{})
	go func() {
		for {
			select {
			case <-ticker.C:
				updatedDocs, err := scanner.Scan()
				if err == nil && len(updatedDocs) > 0 {
					indexer.Build(updatedDocs)
				}
			case <-stopRescan:
				ticker.Stop()
				return
			}
		}
	}()

	// Wait for OS termination signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	fmt.Println("\n [*] Initiating graceful shutdown...")
	close(stopRescan)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Stop(shutdownCtx); err != nil {
		log.Printf("[-] Error during shutdown: %v", err)
	}

	fmt.Println(" [✓] Local AI Worker safely terminated. Zero data leaks.")
}
