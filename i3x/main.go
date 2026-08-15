package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"
)

var (
	// Version can be overwritten at build time via -ldflags="-X main.Version=..."
	Version = "1.0.0"
)

type stringSlice []string

func (s *stringSlice) String() string {
	return strings.Join(*s, ", ")
}

func (s *stringSlice) Set(v string) error {
	*s = append(*s, v)
	return nil
}

func loadEnvFile(paths ...string) {
	for _, p := range paths {
		f, err := os.Open(p)
		if err != nil {
			continue
		}
		defer f.Close()
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				k := strings.TrimSpace(parts[0])
				v := strings.Trim(strings.TrimSpace(parts[1]), "\"'")
				if os.Getenv(k) == "" {
					os.Setenv(k, v)
				}
			}
		}
	}
}

func main() {
	loadEnvFile(".env", "../.env")

	defaultURL := os.Getenv("I3X_URL")
	if defaultURL == "" {
		defaultURL = "https://api.i3x.dev"
	}
	initialClientID := os.Getenv("I3X_CLIENT_ID")
	if initialClientID == "" {
		initialClientID = defaultClientID()
	}
	defaultToken := os.Getenv("I3X_TOKEN")
	defaultAPIKey := os.Getenv("I3X_API_KEY")
	defaultFormat := os.Getenv("I3X_FORMAT")
	if defaultFormat == "" {
		defaultFormat = "table"
	}
	_, defaultNoColor := os.LookupEnv("NO_COLOR")

	var (
		baseURL   string
		clientID  string
		token     string
		apiKey    string
		formatStr string
		noColor   bool
		verbose   bool
		insecure  bool
		timeout   time.Duration
		showVer   bool
		showHelp  bool
		headers   stringSlice
	)

	fs := flag.NewFlagSet("i3x", flag.ContinueOnError)
	fs.StringVar(&baseURL, "url", defaultURL, "i3X Server Base URL (or I3X_URL)")
	fs.StringVar(&baseURL, "u", defaultURL, "i3X Server Base URL (shorthand)")
	fs.StringVar(&clientID, "client-id", initialClientID, "Client ID for scoping subscriptions (or I3X_CLIENT_ID)")
	fs.StringVar(&clientID, "c", initialClientID, "Client ID (shorthand)")
	fs.StringVar(&token, "token", defaultToken, "Bearer authentication token (or I3X_TOKEN)")
	fs.StringVar(&token, "t", defaultToken, "Bearer authentication token (shorthand)")
	fs.StringVar(&apiKey, "api-key", defaultAPIKey, "API Key for X-API-Key header (or I3X_API_KEY)")
	fs.StringVar(&apiKey, "k", defaultAPIKey, "API Key (shorthand)")
	fs.StringVar(&formatStr, "format", defaultFormat, "Output format: table, json, raw, csv, tree")
	fs.StringVar(&formatStr, "o", defaultFormat, "Output format (shorthand)")
	fs.BoolVar(&noColor, "no-color", defaultNoColor, "Disable colored output (or NO_COLOR)")
	fs.BoolVar(&verbose, "verbose", false, "Enable verbose HTTP logging")
	fs.BoolVar(&verbose, "v", false, "Enable verbose HTTP logging (shorthand)")
	fs.BoolVar(&insecure, "insecure", false, "Skip TLS verification")
	fs.DurationVar(&timeout, "timeout", 30*time.Second, "HTTP request timeout")
	fs.BoolVar(&showVer, "version", false, "Show i3x CLI version")
	fs.BoolVar(&showVer, "V", false, "Show i3x CLI version (shorthand)")
	fs.BoolVar(&showHelp, "help", false, "Show help message")
	fs.BoolVar(&showHelp, "h", false, "Show help message (shorthand)")
	fs.Var(&headers, "header", "Custom HTTP Header 'Key: Value' (can be used multiple times)")
	fs.Var(&headers, "H", "Custom HTTP Header (shorthand)")

	fs.Usage = PrintHelp

	// Extract global flags and command arguments separately
	rawArgs := os.Args[1:]
	var flagArgs []string
	var cmdArgs []string

	for i := 0; i < len(rawArgs); i++ {
		arg := rawArgs[i]
		if arg == "-h" || arg == "--help" || arg == "-V" || arg == "--version" || arg == "--no-color" || arg == "--insecure" || arg == "-v" || arg == "--verbose" {
			flagArgs = append(flagArgs, arg)
		} else if arg == "-u" || arg == "--url" || arg == "-c" || arg == "--client-id" || arg == "-t" || arg == "--token" || arg == "-k" || arg == "--api-key" || arg == "-o" || arg == "--format" || arg == "--timeout" || arg == "-H" || arg == "--header" {
			flagArgs = append(flagArgs, arg)
			if i+1 < len(rawArgs) {
				flagArgs = append(flagArgs, rawArgs[i+1])
				i++
			}
		} else if strings.HasPrefix(arg, "--url=") || strings.HasPrefix(arg, "--client-id=") || strings.HasPrefix(arg, "--token=") || strings.HasPrefix(arg, "--api-key=") || strings.HasPrefix(arg, "--format=") || strings.HasPrefix(arg, "-o=") || strings.HasPrefix(arg, "--header=") || strings.HasPrefix(arg, "--timeout=") {
			flagArgs = append(flagArgs, arg)
		} else if arg == "--json" && len(cmdArgs) == 0 {
			formatStr = "json"
		} else if arg == "--raw" && len(cmdArgs) == 0 {
			formatStr = "raw"
		} else if arg == "--csv" && len(cmdArgs) == 0 {
			formatStr = "csv"
		} else if arg == "--tree" && len(cmdArgs) == 0 {
			formatStr = "tree"
		} else {
			cmdArgs = append(cmdArgs, arg)
		}
	}

	if err := fs.Parse(flagArgs); err != nil {
		os.Exit(1)
	}

	if showVer {
		fmt.Printf("i3x CLI v%s (Official i3X 1.0 API Spec)\n", Version)
		return
	}

	if showHelp {
		PrintHelp()
		return
	}

	headerMap := make(map[string]string)
	for _, h := range headers {
		parts := strings.SplitN(h, ":", 2)
		if len(parts) == 2 {
			headerMap[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}

	cfg := ClientConfig{
		BaseURL:   baseURL,
		ClientID:  clientID,
		Token:     token,
		APIKey:    apiKey,
		Headers:   headerMap,
		Timeout:   timeout,
		Insecure:  insecure,
		Verbose:   verbose,
		UserAgent: fmt.Sprintf("i3x-cli/%s", Version),
	}

	client := NewClient(cfg)
	formatter := NewFormatter(OutputFormat(formatStr), noColor)
	handler := NewCommandHandler(client, formatter)

	args := cmdArgs
	if len(args) == 0 {
		// Start interactive REPL shell
		StartREPL(client, formatter)
		return
	}

	ctx := context.Background()
	if err := handler.Execute(ctx, args); err != nil {
		fmt.Fprintf(os.Stderr, "%s %v\n", formatter.color(colorRed+colorBold, "Error:"), err)
		os.Exit(1)
	}
}

// PrintHelp outputs the user documentation and reference guide.
func PrintHelp() {
	fmt.Println(`i3x - Official Command Line Interface for i3X API 1.0 (Industrial Information Interface eXchange)

USAGE:
  i3x [flags] [command] [arguments...]
  i3x                         Launch interactive REPL shell

GLOBAL FLAGS:
  -u, --url <url>             Server base URL (default: https://api.i3x.dev, env: I3X_URL)
  -c, --client-id <id>        Client ID for subscriptions (default: hostname-based, env: I3X_CLIENT_ID)
  -t, --token <token>         Bearer token for Authorization header (env: I3X_TOKEN)
  -k, --api-key <key>         API Key for X-API-Key header (env: I3X_API_KEY)
  -H, --header <key:val>      Custom HTTP header (can specify multiple)
  -o, --format <format>       Output format: table (default), json, raw, csv, tree
      --no-color              Disable ANSI colors
  -v, --verbose               Print verbose HTTP request & response logs
      --insecure              Allow insecure TLS connections (skip certificate verification)
      --timeout <duration>    Request timeout duration (default: 30s)
  -V, --version               Show CLI version
  -h, --help                  Show this help reference

COMMANDS:

  1. SERVER INFO & HEALTH
    info                      Get server version, spec compliance, and capabilities (GET /v1/info)

  2. EXPLORE (NAMESPACES, TYPES & OBJECTS)
    namespaces                List all registered namespaces (GET /v1/namespaces)
    types [options]           List object types schemas (GET /v1/objecttypes)
                                Options: --namespace <uri>
    types query <id...>       Query schema for one or more object types by elementId (POST /v1/objecttypes/query)
    rel-types [options]       List relationship types (GET /v1/relationshiptypes)
                                Options: --namespace <uri>
    rel-types query <id...>   Query relationship types by elementId (POST /v1/relationshiptypes/query)
    objects [options]         List objects (GET /v1/objects)
                                Options: --type <typeId>, --metadata, --root, --non-root
    objects query <id...>     List objects by elementId (POST /v1/objects/list)
                                Options: --metadata
    related <id...>           Query related objects (POST /v1/objects/related)
                                Options: --rel-type <relType>, --metadata

  3. VALUES (READ & WRITE)
    read <id...>              Read last known values (POST /v1/objects/value)
                                Options: --depth <n> (0=infinite, 1=no recursion)
    write <id> <val>          Update current value of object (PUT /v1/objects/value)
                                Options: --quality <q>, --timestamp <rfc3339>
    write id1=val1 id2=val2   Batch write multiple key-value pairs
    write --json '<json>'     Batch write with explicit JSON payload

  4. HISTORY (TIME-SERIES)
    history <id...>           Query historical values (POST /v1/objects/history)
                                Options: --start <rfc3339|-1h|-15m>, --end <rfc3339>, --depth <n>
    write-history <id> <val>  Update historical value of object (PUT /v1/objects/history)
                                Options: --quality <q>, --timestamp <rfc3339>
    write-history --json '<json>' Batch write historical records

  5. SUBSCRIPTIONS (SSE & SYNC)
    sub create [options]      Create a new subscription (POST /v1/subscriptions)
                                Options: --name <displayName>, --client-id <id>
    sub list <subId...>       List and inspect subscriptions (POST /v1/subscriptions/list)
    sub register <subId> <id...> Register objects to monitor (POST /v1/subscriptions/register)
                                Options: --depth <n>
    sub unregister <subId> <id...> Remove objects from monitoring (POST /v1/subscriptions/unregister)
    sub sync <subId>          Acknowledge & sync pending batches (POST /v1/subscriptions/sync)
                                Options: --ack-seq <seq>
    sub stream <subId>        Open live SSE stream for subscription (POST /v1/subscriptions/stream)
    sub delete <subId...>     Delete subscriptions (POST /v1/subscriptions/delete)

  6. HIGH-LEVEL WATCH & MONITORING
    watch <id...>             Create subscription, register items, stream SSE live, and auto-cleanup on exit!
                                Options: --depth <n>, --name <displayName>

EXAMPLES:
  # Check server health & capabilities
  i3x info

  # Explore object types and browse objects
  i3x namespaces
  i3x types
  i3x objects --root --format tree

  # Read and write values
  i3x read pump-station
  i3x write pump-station 42.5 --quality Good
  i3x read pump-station --format json

  # Query historical data from 2 hours ago
  i3x history pump-station --start -2h

  # Stream real-time telemetry live
  i3x watch pump-station

  # Interactive REPL mode
  i3x`)
}
