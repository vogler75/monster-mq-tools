package main

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/chzyer/readline"
)

// ReplState holds active session variables in the REPL.
type ReplState struct {
	ActiveSubID string
}

// StartREPL launches the interactive REPL shell.
func StartREPL(client *Client, formatter *Formatter) {
	state := &ReplState{}
	handler := NewCommandHandler(client, formatter)

	historyFile := ""
	if home, err := os.UserHomeDir(); err == nil {
		historyFile = filepath.Join(home, ".i3x_history")
	}

	completer := readline.NewPrefixCompleter(
		readline.PcItem("info"),
		readline.PcItem("namespaces"),
		readline.PcItem("types",
			readline.PcItem("query"),
		),
		readline.PcItem("rel-types",
			readline.PcItem("query"),
		),
		readline.PcItem("objects",
			readline.PcItem("query"),
			readline.PcItem("related"),
		),
		readline.PcItem("read"),
		readline.PcItem("write"),
		readline.PcItem("history"),
		readline.PcItem("write-history"),
		readline.PcItem("sub",
			readline.PcItem("create"),
			readline.PcItem("list"),
			readline.PcItem("register"),
			readline.PcItem("unregister"),
			readline.PcItem("sync"),
			readline.PcItem("stream"),
			readline.PcItem("delete"),
		),
		readline.PcItem("watch"),
		readline.PcItem("use"),
		readline.PcItem("format",
			readline.PcItem("table"),
			readline.PcItem("json"),
			readline.PcItem("raw"),
			readline.PcItem("csv"),
			readline.PcItem("tree"),
		),
		readline.PcItem("set-url"),
		readline.PcItem("set-client"),
		readline.PcItem("set-token"),
		readline.PcItem("clear"),
		readline.PcItem("help"),
		readline.PcItem("exit"),
		readline.PcItem("quit"),
	)

	rlConfig := &readline.Config{
		Prompt:          getPrompt(client, state, formatter),
		AutoComplete:    completer,
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",
		HistoryFile:     historyFile,
	}

	rl, err := readline.NewEx(rlConfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize interactive shell: %v\n", err)
		return
	}
	defer rl.Close()

	fmt.Println(formatter.color(colorCyan+colorBold, "========================================================="))
	fmt.Println(formatter.color(colorCyan+colorBold, "           i3X Industrial API 1.0 Shell                  "))
	fmt.Println(formatter.color(colorCyan+colorBold, "========================================================="))
	fmt.Printf("Connected to: %s\n", formatter.color(colorGreen, client.cfg.BaseURL))
	fmt.Printf("Client ID:    %s\n", formatter.color(colorYellow, client.cfg.ClientID))
	fmt.Println("Type " + formatter.color(colorBold, "help") + " for command overview, " + formatter.color(colorBold, "exit") + " or Ctrl+D to quit.\n")

	for {
		rl.SetPrompt(getPrompt(client, state, formatter))
		line, err := rl.Readline()
		if err != nil { // EOF or interrupt
			break
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := parseCommandLine(line)
		if len(parts) == 0 {
			continue
		}

		first := strings.ToLower(parts[0])

		if first == "exit" || first == "quit" || first == "q" {
			break
		}

		if first == "clear" || first == "cls" {
			print("\033[H\033[2J")
			continue
		}

		if first == "use" {
			if len(parts) > 1 {
				state.ActiveSubID = parts[1]
				fmt.Printf("Active subscription set to: %s\n", formatter.color(colorCyan, state.ActiveSubID))
			} else {
				fmt.Printf("Current active subscription: %s\n", formatter.color(colorCyan, state.ActiveSubID))
			}
			continue
		}

		if first == "format" || first == "set-format" {
			if len(parts) > 1 {
				f := OutputFormat(strings.ToLower(parts[1]))
				switch f {
				case FormatTable, FormatJSON, FormatRaw, FormatCSV, FormatTree:
					formatter.Format = f
					fmt.Printf("Output format set to: %s\n", f)
				default:
					fmt.Println("Unknown format. Supported: table, json, raw, csv, tree")
				}
			} else {
				fmt.Printf("Current output format: %s\n", formatter.Format)
			}
			continue
		}

		if first == "set-url" {
			if len(parts) > 1 {
				client.SetBaseURL(parts[1])
				fmt.Printf("Target URL updated to: %s\n", parts[1])
			} else {
				fmt.Printf("Current Base URL: %s\n", client.cfg.BaseURL)
			}
			continue
		}

		if first == "set-client" {
			if len(parts) > 1 {
				client.SetClientID(parts[1])
				fmt.Printf("Client ID updated to: %s\n", parts[1])
			} else {
				fmt.Printf("Current Client ID: %s\n", client.cfg.ClientID)
			}
			continue
		}

		if first == "set-token" {
			if len(parts) > 1 {
				client.SetToken(parts[1])
				fmt.Println("Bearer token updated.")
			} else {
				client.SetToken("")
				fmt.Println("Bearer token cleared.")
			}
			continue
		}

		// Inject active subscription ID if user executed sub register/unregister/sync/stream without specifying ID
		if first == "sub" && len(parts) >= 2 && state.ActiveSubID != "" {
			subAction := strings.ToLower(parts[1])
			if (subAction == "register" || subAction == "unregister" || subAction == "sync" || subAction == "stream") && len(parts) == 2 {
				parts = append(parts, state.ActiveSubID)
			}
		}

		ctx := context.Background()
		if err := handler.Execute(ctx, parts); err != nil {
			fmt.Fprintf(os.Stderr, "%s %v\n", formatter.color(colorRed, "Error:"), err)
		}
	}
}

func getPrompt(client *Client, state *ReplState, formatter *Formatter) string {
	host := "i3x"
	if u, err := url.Parse(client.cfg.BaseURL); err == nil && u.Host != "" {
		host = u.Host
	}

	subPart := ""
	if state.ActiveSubID != "" {
		shortSub := state.ActiveSubID
		if len(shortSub) > 8 {
			shortSub = shortSub[:8] + "…"
		}
		subPart = " [" + shortSub + "]"
	}

	return fmt.Sprintf("%s%s> ", host, subPart)
}

func parseCommandLine(cmd string) []string {
	var args []string
	var current strings.Builder
	inQuote := false
	quoteChar := byte(0)

	for i := 0; i < len(cmd); i++ {
		c := cmd[i]
		if inQuote {
			if c == quoteChar {
				inQuote = false
			} else {
				current.WriteByte(c)
			}
		} else {
			if c == '"' || c == '\'' {
				inQuote = true
				quoteChar = c
			} else if c == ' ' || c == '\t' {
				if current.Len() > 0 {
					args = append(args, current.String())
					current.Reset()
				}
			} else {
				current.WriteByte(c)
			}
		}
	}
	if current.Len() > 0 {
		args = append(args, current.String())
	}
	return args
}
