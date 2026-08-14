package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/chzyer/readline"
)

// splitCommandLine splits a command line string into tokens, respecting single and double quotes.
func splitCommandLine(input string) ([]string, error) {
	var tokens []string
	var current strings.Builder
	inSingleQuote := false
	inDoubleQuote := false
	escaped := false

	runes := []rune(strings.TrimSpace(input))
	for i := 0; i < len(runes); i++ {
		r := runes[i]

		if escaped {
			current.WriteRune(r)
			escaped = false
			continue
		}

		if r == '\\' && !inSingleQuote {
			escaped = true
			continue
		}

		if r == '\'' && !inDoubleQuote {
			inSingleQuote = !inSingleQuote
			continue
		}

		if r == '"' && !inSingleQuote {
			inDoubleQuote = !inDoubleQuote
			continue
		}

		if (r == ' ' || r == '\t' || r == '\n' || r == '\r') && !inSingleQuote && !inDoubleQuote {
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
			continue
		}

		current.WriteRune(r)
	}

	if inSingleQuote || inDoubleQuote {
		return nil, fmt.Errorf("unclosed quote in command line")
	}

	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}

	return tokens, nil
}

// normalizeEndpointURL converts flexible inputs (e.g. "4001", "192.168.1.50:4001") to full GraphQL endpoint URLs.
func normalizeEndpointURL(target string) string {
	target = strings.TrimSpace(target)
	if target == "" {
		return "http://localhost:4000/graphql"
	}
	targetClean := strings.TrimPrefix(target, ":")
	if port, err := strconv.Atoi(targetClean); err == nil && port > 0 && port <= 65535 {
		return fmt.Sprintf("http://localhost:%d/graphql", port)
	}

	if !strings.HasPrefix(target, "http://") && !strings.HasPrefix(target, "https://") {
		target = "http://" + target
	}

	u, err := url.Parse(target)
	if err == nil {
		if u.Path == "" || u.Path == "/" {
			u.Path = "/graphql"
		}
		return u.String()
	}

	return target
}

// formatPromptHost returns a short readable endpoint representation for the prompt.
func formatPromptHost(endpointURL string) string {
	u, err := url.Parse(endpointURL)
	if err == nil && u.Host != "" {
		return u.Host
	}
	return endpointURL
}

func getPrompt(client *Client) string {
	hostStr := formatPromptHost(client.cfg.URL)
	modeTag := ""
	if client.cfg.JSONMode {
		modeTag = " (json)"
	}
	return fmt.Sprintf("mmq [%s%s]> ", hostStr, modeTag)
}

// buildCompleter creates a PrefixCompleter supporting tab autocompletion for all commands and subcommands.
func buildCompleter() *readline.PrefixCompleter {
	return readline.NewPrefixCompleter(
		readline.PcItem("searchTopics"),
		readline.PcItem("currentValue"),
		readline.PcItem("currentValues"),
		readline.PcItem("publish"),
		readline.PcItem("retainedMessages"),
		readline.PcItem("browseTopics"),
		readline.PcItem("archivedMessages"),
		readline.PcItem("aggregatedMessages"),
		readline.PcItem("archiveGroups"),
		readline.PcItem("archiveStats"),
		readline.PcItem("systemLogs"),
		readline.PcItem("sessions"),
		readline.PcItem("session",
			readline.PcItem("list"),
			readline.PcItem("inspect"),
			readline.PcItem("remove"),
		),
		readline.PcItem("currentUser"),
		readline.PcItem("databaseConnections"),
		readline.PcItem("hmis"),
		readline.PcItem("hmi",
			readline.PcItem("list"),
			readline.PcItem("create"),
			readline.PcItem("remove"),
			readline.PcItem("export"),
			readline.PcItem("import"),
		),
		readline.PcItem("exportHmiZip"),
		readline.PcItem("importHmiZip"),
		readline.PcItem("brokerConfig"),
		readline.PcItem("features"),
		readline.PcItem("device",
			readline.PcItem("list",
				readline.PcItem("OPCUA_CLIENT"),
				readline.PcItem("MQTT_CLIENT"),
				readline.PcItem("MQTT_SERVER"),
				readline.PcItem("KAFKA_CLIENT"),
			),
			readline.PcItem("download"),
			readline.PcItem("upload"),
			readline.PcItem("enable"),
			readline.PcItem("disable"),
		),
		readline.PcItem("connect"),
		readline.PcItem("auth"),
		readline.PcItem("login"),
		readline.PcItem("token"),
		readline.PcItem("status"),
		readline.PcItem("json",
			readline.PcItem("on"),
			readline.PcItem("off"),
		),
		readline.PcItem("clear"),
		readline.PcItem("help"),
		readline.PcItem("exit"),
		readline.PcItem("quit"),
	)
}

// probeBrokerStatus checks broker connectivity and returns feature list and currentUser if available.
func probeBrokerStatus(ctx context.Context, client *Client) (bool, []string, string, error) {
	ctxTimeout, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	query := `
		query ProbeStatus {
			broker {
				enabledFeatures
			}
			currentUser {
				username
				isAdmin
			}
		}
	`
	var res struct {
		Data struct {
			Broker struct {
				EnabledFeatures []string `json:"enabledFeatures"`
			} `json:"broker"`
			CurrentUser *struct {
				Username string `json:"username"`
				IsAdmin  bool   `json:"isAdmin"`
			} `json:"currentUser"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}

	err := client.rawDo(ctxTimeout, query, nil, &res, client.token != "")
	if err != nil {
		return false, nil, "", err
	}

	userInfo := "Anonymous"
	if res.Data.CurrentUser != nil && res.Data.CurrentUser.Username != "" {
		userInfo = res.Data.CurrentUser.Username
		if res.Data.CurrentUser.IsAdmin {
			userInfo += " (Admin)"
		}
	} else if client.cfg.Username != "" {
		userInfo = client.cfg.Username
	}

	return true, res.Data.Broker.EnabledFeatures, userInfo, nil
}

// printBanner displays the interactive session header and broker connectivity summary.
func printBanner(ctx context.Context, client *Client) {
	fmt.Println("============================================================")
	fmt.Println("  MonsterMQ Interactive CLI (mmq)")
	fmt.Printf("  Endpoint : %s\n", client.cfg.URL)

	online, _, user, err := probeBrokerStatus(ctx, client)
	if online {
		fmt.Println("  Status   : Connected")
		fmt.Printf("  Auth     : %s\n", user)
	} else {
		errMsg := "Offline or unreachable"
		if err != nil {
			errMsg = err.Error()
		}
		fmt.Printf("  Status   : Warning - %s\n", errMsg)
	}
	fmt.Println("============================================================")
	fmt.Println("Type 'help' for commands, 'status' for broker info,")
	fmt.Println("'connect <url|port>' to change endpoint, or 'exit' / 'quit' to exit.")
	fmt.Println()
}

const replHelpText = `Available Commands in MonsterMQ Interactive Shell:

Broker & Topic Operations:
  searchTopics [pattern]                      Search active topics (globs *, SQL %, MQTT #)
  currentValue <topic>                        Get current/retained value for a topic
  currentValues <filter>                      Get current values matching a topic filter
  retainedMessages [filter]                   List retained messages matching a topic filter
  browseTopics [path]                         Browse topic hierarchy level-by-level
  publish <topic> <payload>                   Publish payload (--retain, --qos 0|1|2)
  archivedMessages <topic>                    Query historical time-series messages
  aggregatedMessages <topics...>              Query aggregated time-series metrics
  archiveGroups                               List all deployed archive storage groups
  archiveStats <group>                        Get stats for an archive group
  systemLogs                                  View broker system log entries (--last-minutes N)
  sessions                                    List active MQTT client sessions
  session <clientId>                          Inspect specific client session details
  currentUser                                 Get authenticated user and role info
  databaseConnections                         List configured database connections
  hmis, hmi list                              List deployed HMI web dashboards
  hmi create <name> [options]                 Create a new HMI dashboard definition
  hmi remove <name...>                        Delete and remove one or more HMI dashboards
  exportHmiZip <name> [file.zip]              Export HMI dashboard to a binary zip
  importHmiZip <file.zip> [name]              Import & deploy HMI dashboard from a zip
  brokerConfig (or 'features')                List enabled broker features & capabilities
  device list|download|upload|enable|disable Manage edge devices

Interactive Shell Commands:
  connect <url|host:port|port>                Switch / reconnect to a broker endpoint
  auth <username> <password>                  Authenticate with username and password
  login <username> <password>                 Alias for 'auth'
  token <jwt>                                 Set JWT Bearer token
  status                                      Inspect broker connection, auth, and features
  json [on|off]                               Toggle or inspect raw JSON output mode
  clear                                       Clear the terminal screen
  help [command]                              Show this help menu
  exit, quit                                  Exit interactive shell
`

// RunInteractiveShell starts an interactive REPL session with readline tab completion and history.
func RunInteractiveShell(client *Client, initialCtx context.Context) error {
	printBanner(initialCtx, client)

	historyFile := filepath.Join(os.TempDir(), ".mmq_history")
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		historyFile = filepath.Join(home, ".mmq_history")
	}

	rl, err := readline.NewEx(&readline.Config{
		Prompt:          getPrompt(client),
		AutoComplete:    buildCompleter(),
		HistoryFile:     historyFile,
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",
	})
	if err != nil {
		return runScannerFallbackShell(client, initialCtx)
	}
	defer rl.Close()

	for {
		rl.SetPrompt(getPrompt(client))
		line, err := rl.Readline()
		if err != nil {
			if err == readline.ErrInterrupt {
				if len(line) == 0 {
					return nil
				}
				continue
			} else if err == io.EOF {
				fmt.Println("Bye!")
				return nil
			}
			return err
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		tokens, err := splitCommandLine(line)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Parse error: %v\n", err)
			continue
		}

		if len(tokens) == 0 {
			continue
		}

		cmd := tokens[0]
		args := tokens[1:]

		// Handle REPL-specific builtins
		switch strings.ToLower(cmd) {
		case "exit", "quit", "q":
			fmt.Println("Bye!")
			return nil

		case "clear", "cls":
			fmt.Print("\033[H\033[2J")
			continue

		case "help", "?":
			if len(args) == 0 {
				fmt.Print(replHelpText)
			} else {
				fmt.Print(replHelpText)
			}
			continue

		case "status":
			online, features, user, probeErr := probeBrokerStatus(initialCtx, client)
			fmt.Println("------------------------------------------------------------")
			fmt.Printf("Broker Endpoint : %s\n", client.cfg.URL)
			if online {
				fmt.Printf("Connection State: Online\n")
				fmt.Printf("Authenticated As: %s\n", user)
				if len(features) > 0 {
					fmt.Printf("Broker Features : %s\n", strings.Join(features, ", "))
				} else {
					fmt.Printf("Broker Features : (none reported)\n")
				}
			} else {
				fmt.Printf("Connection State: Offline / Error (%v)\n", probeErr)
			}
			fmt.Printf("JSON Output Mode: %v\n", client.cfg.JSONMode)
			fmt.Println("------------------------------------------------------------")
			continue

		case "connect":
			if len(args) < 1 {
				fmt.Fprintf(os.Stderr, "Usage: connect <url|host:port|port>\nExamples:\n  connect 4001\n  connect 192.168.1.50:4001\n  connect http://localhost:4000/graphql\n")
				continue
			}
			newURL := normalizeEndpointURL(args[0])
			client.cfg.URL = newURL
			// Reset token when switching server unless user explicitly sets it
			client.token = ""
			fmt.Printf("Connecting to %s ...\n", newURL)
			online, _, user, probeErr := probeBrokerStatus(initialCtx, client)
			if online {
				fmt.Printf("Connected! User: %s\n", user)
			} else {
				fmt.Fprintf(os.Stderr, "Warning: cannot reach %s: %v\n", newURL, probeErr)
			}
			rl.SetPrompt(getPrompt(client))
			continue

		case "auth", "login":
			if len(args) < 2 {
				fmt.Fprintf(os.Stderr, "Usage: auth <username> <password>\n")
				continue
			}
			client.cfg.Username = args[0]
			client.cfg.Password = args[1]
			client.token = ""
			fmt.Printf("Authenticating as %s...\n", client.cfg.Username)
			if err := client.EnsureAuthenticated(initialCtx); err != nil {
				fmt.Fprintf(os.Stderr, "Authentication failed: %v\n", err)
			} else {
				fmt.Printf("Successfully authenticated as %s\n", client.cfg.Username)
			}
			continue

		case "token":
			if len(args) < 1 {
				if client.token != "" {
					fmt.Printf("Current token: %s...\n", client.token[:min(len(client.token), 20)])
				} else {
					fmt.Println("No token set.")
				}
				continue
			}
			client.token = args[0]
			fmt.Println("Token updated.")
			continue

		case "json":
			if len(args) == 0 {
				client.cfg.JSONMode = !client.cfg.JSONMode
			} else {
				val := strings.ToLower(args[0])
				client.cfg.JSONMode = (val == "on" || val == "true" || val == "1" || val == "yes")
			}
			fmt.Printf("JSON output mode: %v\n", client.cfg.JSONMode)
			rl.SetPrompt(getPrompt(client))
			continue
		}

		// Dispatch standard CLI command
		err = ExecuteCommand(initialCtx, client, tokens)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
	}
}

// runScannerFallbackShell is a fallback REPL runner when readline cannot initialize.
func runScannerFallbackShell(client *Client, initialCtx context.Context) error {
	scanner := bufio.NewScanner(os.Stdin)

	for {
		fmt.Print(getPrompt(client))

		if !scanner.Scan() {
			if err := scanner.Err(); err != nil && err != io.EOF {
				return err
			}
			fmt.Println("\nExiting MonsterMQ CLI.")
			return nil
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		tokens, err := splitCommandLine(line)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Parse error: %v\n", err)
			continue
		}

		if len(tokens) == 0 {
			continue
		}

		cmd := tokens[0]
		args := tokens[1:]

		switch strings.ToLower(cmd) {
		case "exit", "quit", "q":
			fmt.Println("Bye!")
			return nil
		case "clear", "cls":
			fmt.Print("\033[H\033[2J")
			continue
		case "help", "?":
			fmt.Print(replHelpText)
			continue
		case "status":
			online, features, user, probeErr := probeBrokerStatus(initialCtx, client)
			fmt.Println("------------------------------------------------------------")
			fmt.Printf("Broker Endpoint : %s\n", client.cfg.URL)
			if online {
				fmt.Printf("Connection State: Online\n")
				fmt.Printf("Authenticated As: %s\n", user)
				if len(features) > 0 {
					fmt.Printf("Broker Features : %s\n", strings.Join(features, ", "))
				}
			} else {
				fmt.Printf("Connection State: Offline / Error (%v)\n", probeErr)
			}
			fmt.Printf("JSON Output Mode: %v\n", client.cfg.JSONMode)
			fmt.Println("------------------------------------------------------------")
			continue
		case "connect":
			if len(args) < 1 {
				fmt.Fprintf(os.Stderr, "Usage: connect <url|host:port|port>\n")
				continue
			}
			client.cfg.URL = normalizeEndpointURL(args[0])
			client.token = ""
			fmt.Printf("Connecting to %s ...\n", client.cfg.URL)
			continue
		case "json":
			if len(args) == 0 {
				client.cfg.JSONMode = !client.cfg.JSONMode
			} else {
				val := strings.ToLower(args[0])
				client.cfg.JSONMode = (val == "on" || val == "true" || val == "1" || val == "yes")
			}
			fmt.Printf("JSON output mode: %v\n", client.cfg.JSONMode)
			continue
		}

		err = ExecuteCommand(initialCtx, client, tokens)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
