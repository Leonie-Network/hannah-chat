// hannah-chat — interactive terminal text shell for Hannah Core.
//
// Connects to a running Hannah Core via gRPC and sends text commands,
// the Go equivalent of scripts/hannah_shell.py in the main Hannah repo.
//
// Usage:
//
//	hannah-chat --config config.yaml
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"time"

	"hannah-chat/internal/config"
	"hannah-chat/internal/hannah"
)

func main() {
	cfgPath := flag.String("config", "config.yaml", "path to config.yaml")
	flag.Parse()

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config %s: %v\n", *cfgPath, err)
		os.Exit(1)
	}

	fmt.Printf("Connecting to Hannah Core at %s …\n", cfg.Hannah.Address)
	client, err := hannah.NewClient(cfg.Hannah.Address)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to dial Hannah Core: %v\n", err)
		os.Exit(1)
	}
	defer client.Close()

	readyCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.WaitReady(readyCtx); err != nil {
		fmt.Fprintf(os.Stderr, "Error: could not reach Hannah Core at %s\n", cfg.Hannah.Address)
		os.Exit(1)
	}

	// Ctrl+C prints a trailing newline before exiting, matching the Ctrl+D path below
	// (scanner.Scan() returning false) instead of leaving the "You: " prompt dangling.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	go func() {
		<-sigCh
		fmt.Println()
		os.Exit(0)
	}()

	fmt.Println("Connected. Type your command (Ctrl+C or Ctrl+D to quit).")
	fmt.Println()

	scanner := bufio.NewScanner(os.Stdin)
	s := newSession(client, scanner)
	for {
		fmt.Print("You: ")
		if !scanner.Scan() {
			fmt.Println()
			break
		}

		text := strings.TrimSpace(scanner.Text())
		if text == "" {
			continue
		}

		if strings.HasPrefix(text, "/") {
			if !dispatchLocalCommand(s, text) {
				fmt.Printf("Unknown command: %s\n\n", strings.Fields(text)[0])
			}
			continue
		}

		resp, err := client.SubmitText(context.Background(), text, s.sourceService, s.sourceUserID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "gRPC error: %v\n", err)
			continue
		}
		fmt.Printf("Hannah [%s]: %s\n\n", resp.IntentName, resp.Answer)
	}
}
