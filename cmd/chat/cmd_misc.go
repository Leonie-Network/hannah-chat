package main

import (
	"os"
)

func init() {
	registerCommand(localCommand{
		name:        "exit",
		help:        "exit the chat client",
		trustLevel: 0,
		run:         cmdExit,
	})
}

func cmdExit(s *session) {
	os.Exit(0)
}