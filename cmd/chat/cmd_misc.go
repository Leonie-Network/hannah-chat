package main

import (
	"fmt"
	"os"

	"github.com/fatih/color"
)

func init() {
	registerCommand(localCommand{
		name:       "exit",
		help:       "exit the chat client",
		trustLevel: 0,
		run:        cmdExit,
	})
	registerCommand(localCommand{
		name:       "help",
		help:       "show this help message",
		trustLevel: 0,
		run:        cmdHelp,
	})
}

func cmdExit(s *session) {
	os.Exit(0)
}

func cmdHelp(s *session) {
	fmt.Println("Available commands:")
	cmdColor := color.New(color.FgCyan)
	for _, cmd := range registry {
		if s.trustLevel >= cmd.trustLevel {
			fmt.Print("  ")
			cmdColor.Printf("/%-6s", cmd.name)
			fmt.Printf(": %s\n", cmd.help)
		}
	}
	fmt.Println()
}
