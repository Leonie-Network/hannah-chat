package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/term"
)

// localCommand is a "/name ..." line handled entirely client-side — never sent
// to Hannah as text. Add new ones by appending to localCommands below; nothing
// else needs to change to make a new command reachable from the prompt.
type localCommand struct {
	name string // without the leading "/"
	help string
	run  func(s *session)
}

var localCommands = []localCommand{
	{
		name: "login",
		help: "log in as a Hannah user, so Hannah knows who it's talking to",
		run:  cmdLogin,
	},
}

// dispatchLocalCommand runs the local command named in line (its first
// whitespace-separated token, leading "/" stripped). Returns false if line
// doesn't name a known command.
func dispatchLocalCommand(s *session, line string) bool {
	name := strings.TrimPrefix(strings.Fields(line)[0], "/")
	for _, c := range localCommands {
		if c.name == name {
			c.run(s)
			return true
		}
	}
	return false
}

func cmdLogin(s *session) {
	fmt.Print("Username: ")
	if !s.scanner.Scan() {
		return
	}
	username := strings.TrimSpace(s.scanner.Text())

	fmt.Print("Password: ")
	passwordBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		fmt.Fprintf(os.Stderr, "could not read password: %v\n\n", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := s.client.Login(ctx, username, string(passwordBytes))
	if err != nil {
		fmt.Fprintf(os.Stderr, "login failed: %v\n\n", err)
		return
	}
	if !resp.Found {
		fmt.Println("Invalid credentials.")
		fmt.Println()
		return
	}

	// Self-link: this roomie's own ID as its "chat" account, so later SubmitText
	// calls with source_service="chat"/source_user_id=<id> resolve back to them
	// (gessinger/voice/hannah#332). Idempotent — logging in again just replaces it.
	userID := strconv.Itoa(int(resp.User.Id))
	linkResp, err := s.client.LinkAccount(ctx, resp.User.Id, "chat", userID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "login succeeded, but linking failed: %v\n\n", err)
		return
	}
	if !linkResp.Ok {
		fmt.Fprintf(os.Stderr, "login succeeded, but linking failed: %s\n\n", linkResp.Message)
		return
	}

	s.sourceUserID = userID

	name := resp.User.DisplayName
	if name == "" {
		name = resp.User.UserName
	}
	fmt.Printf("Logged in as %s.\n\n", name)
}
