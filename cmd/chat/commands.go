package main

import (
	"fmt"
	"strings"
)

// localCommand is a "/name ..." line handled entirely client-side — never sent
// to Hannah as text. Add new ones by appending to localCommands below; nothing
// else needs to change to make a new command reachable from the prompt.
type localCommand struct {
	name       string // without the leading "/"
	help       string
	trustLevel int
	run        func(s *session)
}

var registry = make(map[string]localCommand)

func registerCommand(cmd localCommand) {
	if _, exists := registry[cmd.name]; exists {
		panic(fmt.Sprintf("command %q double registered", cmd.name))
	}
	registry[cmd.name] = cmd
}

// dispatchLocalCommand runs the local command named in line (its first
// whitespace-separated token, leading "/" stripped). Returns false if line
// doesn't name a known command.
func dispatchLocalCommand(s *session, line string) bool {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return false
	}

	name := strings.TrimPrefix(fields[0], "/")
	cmd, exists := registry[name]
	if !exists {
		return false
	}

	// Check trust level
	if s.trustLevel < cmd.trustLevel {
		fmt.Printf("Command '/%s' requires higher permissions (required: %d, current: %d).\n\n",
			name, cmd.trustLevel, s.trustLevel)
		return true // command exists, but permissions are insufficient
	}

	cmd.run(s)
	return true
}
