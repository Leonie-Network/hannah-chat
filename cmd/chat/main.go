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
	"math/rand"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/fatih/color"

	"hannah-chat/internal/config"
	"hannah-chat/internal/hannah"
)

var startupTips = []string{
	"Type /help to see all available commands.",
	"You can log in with /login to access user-specific features.",
	"Use Ctrl+C or Ctrl+D at any time to disconnect cleanly.",
	"Logged-in users get access to device control via /devices.",
}

func printStartupTip() {
	dimmed := color.New(color.FgHiBlack, color.Italic).SprintFunc()

	tip := startupTips[rand.Intn(len(startupTips))]

	fmt.Printf("%s\n\n", dimmed("Tip: "+tip))
}

// bannerGlyphs is a 5-row block font, one entry per letter used in "HANNAH".
// Each glyph is 5 characters wide so rows line up regardless of which
// letters they spell — no hand-counted spacing to get wrong.
var bannerGlyphs = map[byte][5]string{
	'H': {
		"█   █",
		"█   █",
		"█████",
		"█   █",
		"█   █",
	},
	'A': {
		" ███ ",
		"█   █",
		"█████",
		"█   █",
		"█   █",
	},
	'N': {
		"█   █",
		"██  █",
		"█ █ █",
		"█  ██",
		"█   █",
	},
}

// bannerText renders word as block-letter ASCII art, one space between glyphs.
func bannerText(word string) string {
	rows := make([]string, 5)
	for i := 0; i < len(word); i++ {
		glyph := bannerGlyphs[word[i]]
		for r := range rows {
			if i > 0 {
				rows[r] += " "
			}
			rows[r] += glyph[r]
		}
	}
	return strings.Join(rows, "\n")
}

func printBanner() {
	cyan := color.New(color.FgCyan, color.Bold).SprintFunc()
	dimmed := color.New(color.FgHiBlack).SprintFunc()

	fmt.Println(cyan(bannerText("HANNAH")) + dimmed("  | Smart Assistant"))
	fmt.Println()
}

func main() {
	printBanner()

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

	color.Green("Connected. Type your command (Ctrl+C or Ctrl+D to quit).")
	fmt.Println()

	printStartupTip()

	scanner := bufio.NewScanner(os.Stdin)
	s := newSession(client, scanner)

	for {
		fmt.Print(s.name + ": ")
		if !scanner.Scan() {
			fmt.Println()
			break
		}

		text := strings.TrimSpace(scanner.Text())
		if text == "" {
			continue
		}

		// A running /devices menu is modal: every line is a menu selection,
		// not a command or a message to Hannah, until the menu is closed.
		if s.menu != nil {
			handleMenuInput(s, text)
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
