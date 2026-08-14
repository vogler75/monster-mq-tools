package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
)

const usageText = `mmq - Command-line tool for MonsterMQ Broker GraphQL Interface

Usage:
  mmq [global options] [command] [command args]
  mmq [global options] shell                Start interactive REPL CLI session

Global Options:
  --url URL           GraphQL endpoint URL (env: MQ_URL / GRAPHQL_URL, default: http://localhost:4000/graphql)
  --host HOST         Broker host or IP address (env: MQ_HOST / GRAPHQL_HOST, default: localhost)
  --port PORT         Broker port number (env: MQ_PORT / GRAPHQL_PORT, default: 4000)
  --https             Use HTTPS protocol instead of HTTP (env: MQ_HTTPS / GRAPHQL_HTTPS)
  --user USERNAME     Username for auth (env: MQ_USER / GRAPHQL_USER)
  --pass PASSWORD     Password for auth (env: MQ_PASS / GRAPHQL_PASS)
  --token TOKEN       JWT Bearer token (env: MQ_TOKEN / GRAPHQL_TOKEN)
  --env FILE          Path to .env file (default: .env)
  --json              Output raw JSON results
  -i, --interactive   Start interactive REPL CLI session
  -h, --help          Show help usage

Commands:
  shell, repl                                 Start interactive REPL CLI session
  searchTopics [pattern]                      Search active topics (globs *, SQL %, MQTT #)
  currentValue <topic>                        Get current/retained value for a topic
  currentValues <filter>                      Get current values matching a topic filter
  retainedMessages [filter]                   List retained messages matching a topic filter
  browseTopics [path]                         Browse topic hierarchy level-by-level
  publish <topic> <payload>                   Publish payload to a topic (--retain, --qos)
  archivedMessages <topic>                    Query historical time-series messages
  aggregatedMessages <topics...>              Query aggregated time-series metric data
  archiveGroups                               List all deployed archive storage groups
  archiveStats <group>                        Get stats for an archive group
  systemLogs                                  View broker system log entries
  sessions                                    List active MQTT client sessions
  session <clientId>                          Inspect specific client session details
  currentUser                                 Get authenticated user and role info
  databaseConnections                         List configured database connections
  hmis, hmi list                              List deployed HMI web dashboards
  hmi create <name> [options]                 Create a new HMI dashboard definition
  hmi remove <name...>                        Delete and remove one or more HMI dashboards
  exportHmiZip <name> [file.zip]              Export HMI dashboard to a binary zip
  importHmiZip <file.zip> [name]              Import & deploy HMI dashboard from a zip
  brokerConfig                                List enabled broker features & capabilities
  device list|download|upload|enable|disable Manage edge devices

Examples:
  mmq                                                       # Start interactive CLI session on localhost:4000
  mmq --port 4001                                           # Connect to localhost:4001
  mmq --host 192.168.1.50 --port 4001                       # Connect to 192.168.1.50:4001
  mmq --host secure-broker --https                          # Connect via HTTPS to secure-broker:4000
  mmq --url http://192.168.1.50:4000/graphql shell          # Interactive CLI on remote broker
  mmq searchTopics "*Watt*"
  mmq currentValue sensors/temp/room1
  mmq publish sensors/temp/room1 '{"temp": 22.5}' --retain
  mmq archivedMessages sensors/temp/room1 --last-seconds 3600
  mmq brokerConfig
`

func main() {
	var flagURL, flagHost, flagUser, flagPass, flagToken, envFile string
	var flagPort int
	var flagHTTPS, jsonMode, helpFlag, interactiveFlag bool

	fs := flag.NewFlagSet("mmq", flag.ContinueOnError)
	fs.StringVar(&flagURL, "url", "", "GraphQL endpoint URL")
	fs.StringVar(&flagHost, "host", "", "Broker host or IP address")
	fs.IntVar(&flagPort, "port", 0, "Broker port number")
	fs.BoolVar(&flagHTTPS, "https", false, "Use HTTPS protocol")
	fs.StringVar(&flagUser, "user", "", "Username for authentication")
	fs.StringVar(&flagUser, "username", "", "Username for authentication")
	fs.StringVar(&flagPass, "pass", "", "Password for authentication")
	fs.StringVar(&flagPass, "password", "", "Password for authentication")
	fs.StringVar(&flagToken, "token", "", "JWT Bearer token")
	fs.StringVar(&envFile, "env", ".env", "Path to .env file")
	fs.StringVar(&envFile, "env-file", ".env", "Path to .env file")
	fs.BoolVar(&jsonMode, "json", false, "Output raw JSON")
	fs.BoolVar(&interactiveFlag, "i", false, "Start interactive shell")
	fs.BoolVar(&interactiveFlag, "interactive", false, "Start interactive shell")
	fs.BoolVar(&helpFlag, "help", false, "Show help usage")
	fs.BoolVar(&helpFlag, "h", false, "Show help usage")

	// Extract global flags vs subcommands
	var globalArgs []string
	var commandArgs []string

	for i := 1; i < len(os.Args); i++ {
		arg := os.Args[i]
		if strings.HasPrefix(arg, "-") && len(commandArgs) == 0 {
			globalArgs = append(globalArgs, arg)
			if (arg == "--url" || arg == "--host" || arg == "--port" || arg == "--user" || arg == "--username" || arg == "--pass" || arg == "--password" || arg == "--token" || arg == "--env" || arg == "--env-file") && i+1 < len(os.Args) {
				i++
				globalArgs = append(globalArgs, os.Args[i])
			}
		} else {
			commandArgs = append(commandArgs, arg)
		}
	}

	_ = fs.Parse(globalArgs)

	if helpFlag {
		fmt.Print(usageText)
		os.Exit(0)
	}

	cfg := ResolveClientConfig(flagURL, flagHost, flagPort, flagHTTPS, flagUser, flagPass, flagToken, envFile, jsonMode)
	client := NewClient(cfg)
	ctx := context.Background()

	// If interactive flag is set or no command arguments provided, run interactive shell
	if interactiveFlag || len(commandArgs) == 0 || (len(commandArgs) > 0 && isShellCommand(commandArgs[0])) {
		if err := RunInteractiveShell(client, ctx); err != nil {
			fmt.Fprintf(os.Stderr, "Shell error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if err := ExecuteCommand(ctx, client, commandArgs); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func isShellCommand(cmd string) bool {
	switch strings.ToLower(cmd) {
	case "shell", "repl", "interactive", "sh":
		return true
	default:
		return false
	}
}

// ExecuteCommand executes a single command with arguments against the MonsterMQ client.
func ExecuteCommand(ctx context.Context, client *Client, commandArgs []string) error {
	if len(commandArgs) == 0 {
		return nil
	}

	subcmd := commandArgs[0]
	subargs := commandArgs[1:]

	switch subcmd {
	case "currentValue":
		return runGetValue(ctx, client, subargs)
	case "currentValues":
		return runGetValues(ctx, client, subargs)
	case "publish":
		return runSetValue(ctx, client, subargs)
	case "searchTopics":
		return runListTopics(ctx, client, subargs)
	case "browseTopics":
		return runBrowseTopics(ctx, client, subargs)
	case "retainedMessages":
		return runListRetained(ctx, client, subargs)
	case "archiveGroups":
		return runListArchives(ctx, client, subargs)
	case "archiveStats":
		return runArchiveStats(ctx, client, subargs)
	case "archivedMessages":
		return runQueryHistory(ctx, client, subargs)
	case "aggregatedMessages":
		return runQueryAggregated(ctx, client, subargs)
	case "systemLogs":
		return runLogs(ctx, client, subargs)
	case "hmis", "hmi":
		if len(subargs) == 0 {
			return runHmiList(ctx, client, nil)
		}
		action := subargs[0]
		actionArgs := subargs[1:]
		switch strings.ToLower(action) {
		case "-h", "--help", "help":
			fmt.Println("Usage: mmq hmis")
			fmt.Println("       mmq hmi list")
			fmt.Println("       mmq hmi create <name> [options]")
			fmt.Println("       mmq hmi remove <name1> [name2...]")
			fmt.Println("       mmq exportHmiZip <name> [output.zip]")
			fmt.Println("       mmq importHmiZip <file.zip> [name] [--main]")
			fmt.Println()
			fmt.Println("Manage deployed HMI dashboards and web packages.")
			fmt.Println()
			fmt.Println("Commands:")
			fmt.Println("  hmi list                     List all deployed HMI dashboards (alias: hmis)")
			fmt.Println("  hmi create <name> [options]  Create a new HMI dashboard definition")
			fmt.Println("  hmi remove <name...>         Delete and remove one or more HMI dashboards")
			fmt.Println("  exportHmiZip <name>          Export HMI dashboard to a binary zip file")
			fmt.Println("  importHmiZip <file.zip>      Import & deploy HMI dashboard from a zip package")
			fmt.Println()
			fmt.Println("Options:")
			fmt.Println("  -h, --help                   Show this help text")
			return nil
		case "list", "ls":
			return runHmiList(ctx, client, actionArgs)
		case "create", "new", "add":
			return runHmiCreate(ctx, client, actionArgs)
		case "remove", "delete", "rm":
			return runHmiRemove(ctx, client, actionArgs)
		case "export":
			return runExportHmiZip(ctx, client, actionArgs)
		case "import":
			return runImportHmiZip(ctx, client, actionArgs)
		default:
			return runHmiList(ctx, client, subargs)
		}
	case "sessions":
		if hasHelpFlag(subargs) {
			fmt.Println("Usage: mmq sessions")
			fmt.Println()
			fmt.Println("List all active MQTT client sessions.")
			fmt.Println()
			fmt.Println("Options:")
			fmt.Println("  -h, --help   Show this help text")
			return nil
		}
		return runSessionList(ctx, client, subargs)
	case "session":
		if len(subargs) == 0 {
			return runSessionList(ctx, client, nil)
		}
		action := subargs[0]
		actionArgs := subargs[1:]
		switch strings.ToLower(action) {
		case "-h", "--help", "help":
			fmt.Println("Usage: mmq session [command|clientId]")
			fmt.Println()
			fmt.Println("Manage MQTT client sessions and subscriptions.")
			fmt.Println()
			fmt.Println("Commands:")
			fmt.Println("  session list                 List all active client sessions (alias: sessions)")
			fmt.Println("  session <clientId>           Inspect session details for a specific client")
			fmt.Println("  session inspect <clientId>   Inspect session details for a specific client")
			fmt.Println("  session remove <clientId...> Remove / disconnect one or more client sessions")
			fmt.Println()
			fmt.Println("Options:")
			fmt.Println("  -h, --help                   Show this help text")
			return nil
		case "list":
			return runSessionList(ctx, client, actionArgs)
		case "inspect":
			return runSessionInspect(ctx, client, actionArgs)
		case "remove":
			return runSessionRemove(ctx, client, actionArgs)
		default:
			// Treat argument directly as <clientId> to inspect
			return runSessionInspect(ctx, client, subargs)
		}
	case "features", "brokerConfig":
		return runListFeatures(ctx, client, subargs)
	case "currentUser":
		return runCurrentUser(ctx, client, subargs)
	case "databaseConnections":
		return runDatabaseConnections(ctx, client, subargs)
	case "exportHmiZip":
		return runExportHmiZip(ctx, client, subargs)
	case "importHmiZip":
		return runImportHmiZip(ctx, client, subargs)
	case "device":
		if len(subargs) == 0 {
			return runDeviceList(ctx, client, nil)
		}
		action := subargs[0]
		actionArgs := subargs[1:]
		switch action {
		case "list":
			return runDeviceList(ctx, client, actionArgs)
		case "download":
			return runDeviceDownload(ctx, client, actionArgs)
		case "upload":
			return runDeviceUpload(ctx, client, actionArgs)
		case "enable":
			return runDeviceEnable(ctx, client, actionArgs)
		case "disable":
			return runDeviceDisable(ctx, client, actionArgs)
		default:
			return fmt.Errorf("unknown device action '%s' (use 'list', 'download', 'upload', 'enable', or 'disable')", action)
		}
	default:
		return fmt.Errorf("unknown command '%s'. Run 'mmq --help' or 'help' for usage", subcmd)
	}
}
