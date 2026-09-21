package main

import (
	"bufio"

	"hannah-chat/internal/hannah"
)

// session holds everything a local command needs: the live client, the shared
// stdin scanner (so a command can prompt for more input mid-flow, like /login
// does for username/password), and the identity used for every SubmitText call.
type session struct {
	client  *hannah.Client
	scanner *bufio.Scanner

	// sourceService/sourceUserID are sent with every SubmitText call. Default to
	// the anonymous "chat" identity; /login overwrites sourceUserID with the
	// authenticated roomie's own ID once self-linked (gessinger/voice/hannah#332).
	sourceService string
	sourceUserID  string
}

func newSession(client *hannah.Client, scanner *bufio.Scanner) *session {
	return &session{
		client:        client,
		scanner:       scanner,
		sourceService: "chat",
		sourceUserID:  "chat",
	}
}
