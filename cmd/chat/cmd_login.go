package main

import (
	"context"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/term"
)

func init() {
	registerCommand(localCommand{
		name:        "login",
		help:        "log in as a Hannah user",
		trustLevel: 0,
		run:         cmdLogin,
	})

	registerCommand(localCommand{
		name:        "logout",
		help:        "log out, so Hannah forgets who it's talking to",
		trustLevel: 0,
		run:         cmdLogout,
	})
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
	userID := strconv.Itoa(int(resp.User.Id))
	if _, ok := resp.User.GetLinkedAccounts()["chat"]; !ok {
		linkResp, err := s.client.LinkAccount(ctx, resp.User.Id, "chat", userID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "login succeeded, but linking failed: %v\n\n", err)
			return
		}
		if !linkResp.Ok {
			fmt.Fprintf(os.Stderr, "login succeeded, but linking failed: %s\n\n", linkResp.Message)
			return
		}
	}

	s.sourceUserID = userID
	s.trustLevel = int(resp.User.TrustLevel)

	name := resp.User.DisplayName
	if name == "" {
		name = resp.User.UserName
	}
	fmt.Printf("Logged in as %s.\n\n", name)
}

func cmdLogout(s *session) {
	s.sourceUserID = ""
	s.trustLevel = 0
	fmt.Println("Logged out.")
}