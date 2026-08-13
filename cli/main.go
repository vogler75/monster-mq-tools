package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
)

const usageText = `mmqcli - Command-line tool for MonsterMQ Broker GraphQL Interface

Usage:
  mmqcli [global options] <command> [command args]

Global Options:
  --url URL           GraphQL endpoint URL (env: MQ_URL / GRAPHQL_URL, default: http://localhost:4000/graphql)
  --user USERNAME     Username for auth (env: MQ_USER / GRAPHQL_USER)
  --pass PASSWORD     Password for auth (env: MQ_PASS / GRAPHQL_PASS)
  --token TOKEN       JWT Bearer token (env: MQ_TOKEN / GRAPHQL_TOKEN)
  --env FILE          Path to .env file (default: .env)
  --json              Output raw JSON results

Commands:
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
  hmis                                        List deployed HMI web dashboards
  exportHmiZip <name>                         Export HMI dashboard package
  brokerConfig                                List enabled broker features & capabilities
  device list|download|upload|enable|disable Manage edge devices

Examples:
  mmqcli --url http://localhost:4000/graphql searchTopics "*Watt*"
  mmqcli --url http://localhost:4000/graphql currentValue sensors/temp/room1
  mmqcli publish sensors/temp/room1 '{"temp": 22.5}' --retain
  mmqcli archivedMessages sensors/temp/room1 --last-seconds 3600
  mmqcli brokerConfig
`

func main() {
	if len(os.Args) < 2 {
		fmt.Print(usageText)
		os.Exit(1)
	}

	var flagURL, flagUser, flagPass, flagToken, envFile string
	var jsonMode, helpFlag bool

	fs := flag.NewFlagSet("mmqcli", flag.ContinueOnError)
	fs.StringVar(&flagURL, "url", "", "GraphQL endpoint URL")
	fs.StringVar(&flagUser, "user", "", "Username for authentication")
	fs.StringVar(&flagUser, "username", "", "Username for authentication")
	fs.StringVar(&flagPass, "pass", "", "Password for authentication")
	fs.StringVar(&flagPass, "password", "", "Password for authentication")
	fs.StringVar(&flagToken, "token", "", "JWT Bearer token")
	fs.StringVar(&envFile, "env", ".env", "Path to .env file")
	fs.StringVar(&envFile, "env-file", ".env", "Path to .env file")
	fs.BoolVar(&jsonMode, "json", false, "Output raw JSON")
	fs.BoolVar(&helpFlag, "help", false, "Show help usage")
	fs.BoolVar(&helpFlag, "h", false, "Show help usage")

	// Extract global flags vs subcommands
	var globalArgs []string
	var commandArgs []string

	for i := 1; i < len(os.Args); i++ {
		arg := os.Args[i]
		if strings.HasPrefix(arg, "-") && len(commandArgs) == 0 {
			globalArgs = append(globalArgs, arg)
			if (arg == "--url" || arg == "--user" || arg == "--username" || arg == "--pass" || arg == "--password" || arg == "--token" || arg == "--env" || arg == "--env-file") && i+1 < len(os.Args) {
				i++
				globalArgs = append(globalArgs, os.Args[i])
			}
		} else {
			commandArgs = append(commandArgs, arg)
		}
	}

	_ = fs.Parse(globalArgs)

	if helpFlag || len(commandArgs) == 0 {
		fmt.Print(usageText)
		os.Exit(0)
	}

	subcmd := commandArgs[0]
	subargs := commandArgs[1:]

	cfg := ResolveClientConfig(flagURL, flagUser, flagPass, flagToken, envFile, jsonMode)
	client := NewClient(cfg)
	ctx := context.Background()

	var err error
	switch subcmd {
	case "get-value", "get", "currentValue":
		err = runGetValue(ctx, client, subargs)
	case "get-values", "current-values", "currentValues":
		err = runGetValues(ctx, client, subargs)
	case "set-value", "publish":
		err = runSetValue(ctx, client, subargs)
	case "list-topics", "search-topics", "find-topics", "searchTopics":
		err = runListTopics(ctx, client, subargs)
	case "browse-topics", "browse", "browseTopics":
		err = runBrowseTopics(ctx, client, subargs)
	case "list-retained", "retained", "retainedMessages":
		err = runListRetained(ctx, client, subargs)
	case "list-archives", "archiveGroups":
		err = runListArchives(ctx, client, subargs)
	case "archive-stats", "archiveStats":
		err = runArchiveStats(ctx, client, subargs)
	case "query-history", "history", "archivedMessages":
		err = runQueryHistory(ctx, client, subargs)
	case "query-aggregated", "aggregated", "aggregatedMessages":
		err = runQueryAggregated(ctx, client, subargs)
	case "logs", "system-logs", "systemLogs":
		err = runLogs(ctx, client, subargs)
	case "hmi", "hmis":
		err = runHmiList(ctx, client, subargs)
	case "session", "sessions":
		if len(subargs) == 0 {
			err = runSessionList(ctx, client, nil)
		} else {
			action := subargs[0]
			actionArgs := subargs[1:]
			switch action {
			case "list", "ls":
				err = runSessionList(ctx, client, actionArgs)
			case "inspect", "get", "show":
				err = runSessionInspect(ctx, client, actionArgs)
			case "remove", "delete", "rm", "kill":
				err = runSessionRemove(ctx, client, actionArgs)
			default:
				err = fmt.Errorf("unknown session action '%s' (use 'list', 'inspect', or 'remove')", action)
			}
		}
	case "features", "enabled-features", "list-features", "brokerConfig":
		err = runListFeatures(ctx, client, subargs)
	case "currentUser", "current-user", "whoami":
		err = runCurrentUser(ctx, client, subargs)
	case "databaseConnections", "database-connections", "db":
		err = runDatabaseConnections(ctx, client, subargs)
	case "exportHmiZip", "export-hmi-zip":
		err = runExportHmiZip(ctx, client, subargs)
	case "device", "devices":
		if len(subargs) == 0 {
			err = runDeviceList(ctx, client, nil)
		} else {
			action := subargs[0]
			actionArgs := subargs[1:]
			switch action {
			case "list", "ls":
				err = runDeviceList(ctx, client, actionArgs)
			case "download", "export":
				err = runDeviceDownload(ctx, client, actionArgs)
			case "upload", "import":
				err = runDeviceUpload(ctx, client, actionArgs)
			case "enable", "start":
				err = runDeviceEnable(ctx, client, actionArgs)
			case "disable", "stop":
				err = runDeviceDisable(ctx, client, actionArgs)
			default:
				err = fmt.Errorf("unknown device action '%s' (use 'list', 'download', 'upload', 'enable', or 'disable')", action)
			}
		}
	default:
		err = fmt.Errorf("unknown command '%s'. Run 'mmqcli --help' for usage", subcmd)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
