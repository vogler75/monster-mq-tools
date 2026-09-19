package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type ScriptPublishedMessage struct {
	Topic   string `json:"topic"`
	Payload string `json:"payload"`
	QoS     int    `json:"qos"`
	Retain  bool   `json:"retain"`
}

type ScriptConfigInput struct {
	Language            string   `json:"language"`
	Script              string   `json:"script"`
	TriggerType         string   `json:"triggerType"`
	TopicFilters        []string `json:"topicFilters"`
	TriggerOnChangeOnly bool     `json:"triggerOnChangeOnly"`
	TimerIntervalMs     int      `json:"timerIntervalMs"`
	InstanceMode        string   `json:"instanceMode"`
	TimeoutMs           int      `json:"timeoutMs"`
	Description         string   `json:"description,omitempty"`
}

type ScriptInput struct {
	Name      string            `json:"name"`
	Namespace string            `json:"namespace"`
	NodeID    string            `json:"nodeId"`
	Enabled   bool              `json:"enabled"`
	Config    ScriptConfigInput `json:"config"`
}

type ScriptData struct {
	Name                string   `json:"name"`
	Namespace           string   `json:"namespace"`
	NodeID              string   `json:"nodeId"`
	Enabled             bool     `json:"enabled"`
	CreatedAt           string   `json:"createdAt"`
	UpdatedAt           string   `json:"updatedAt"`
	IsOnCurrentNode     bool     `json:"isOnCurrentNode"`
	ExecutionCount      int64    `json:"executionCount"`
	ErrorCount          int64    `json:"errorCount"`
	LastExecutionTime   *string  `json:"lastExecutionTime"`
	LastExecutionStatus *string  `json:"lastExecutionStatus"`
	RecentLogs          []string `json:"recentLogs"`
	Config              struct {
		Language            string   `json:"language"`
		Script              string   `json:"script"`
		TriggerType         string   `json:"triggerType"`
		TopicFilters        []string `json:"topicFilters"`
		TriggerOnChangeOnly bool     `json:"triggerOnChangeOnly"`
		TimerIntervalMs     int      `json:"timerIntervalMs"`
		InstanceMode        string   `json:"instanceMode"`
		TimeoutMs           int      `json:"timeoutMs"`
		Description         *string  `json:"description"`
	} `json:"config"`
}

type ScriptTestResult struct {
	Success         bool                     `json:"success"`
	ReturnValue     *string                  `json:"returnValue"`
	OutputMessages  []ScriptPublishedMessage `json:"outputMessages"`
	Logs            []string                 `json:"logs"`
	Errors          []string                 `json:"errors"`
	ExecutionTimeMs float64                  `json:"executionTimeMs"`
}

type ScriptLanguageData struct {
	Name          string  `json:"name"`
	DisplayName   string  `json:"displayName"`
	Description   *string `json:"description"`
	IsDefault     *bool   `json:"isDefault"`
	Documentation string  `json:"documentation"`
	Skill         string  `json:"skill"`
}

const scriptHelp = `Usage: mmq script <command> [arguments] [options]
       mmq scripts [filter]

Manage broker scripts, execute dry-run test sandboxes, inspect logs,
and retrieve language documentation and AI skills from the broker.

Commands:
  list [filter]              List configured scripts (alias: mmq scripts)
  get <name>                 Inspect script details, configuration, and code
  create <name> [options]    Create and deploy a new script
  update <name> [options]    Update an existing script configuration or code
  delete <name...>           Delete one or more scripts
  toggle <name> <on|off>     Enable or disable a script (alias: enable, disable)
  start <name>               Start / enable a script
  stop <name>                Stop / disable a script
  test <name|--code> [opts]  Execute dry-run test sandbox with simulated inputs
  logs <name>                Inspect recent circular execution logs
  docs [language]            Get full API reference documentation for a language
  skill [language] [opts]    Get ready-to-use AI Skill (Markdown) for AI coding agents
  languages                  List supported scripting languages on connected broker

Script Options:
  --lang <lang>              Script language (e.g. starlark, python, javascript)
  --trigger <type>           Trigger type: TOPIC, TIMER, BOTH, CALLABLE (default: TOPIC)
  --topic <filter>           Topic filter (can be repeated or comma-separated)
  --interval <ms>            Timer interval in milliseconds (for TIMER/BOTH)
  --on-change                Trigger on change only (skip duplicate consecutive payloads)
  --mode <mode>              Instance mode: SINGLETON (default) or MULTI_INSTANCE
  --timeout <ms>             Execution timeout in ms (default: 200)
  --code <code>              Inline script code
  --file <path>              Read script code from a local file
  --desc <description>       Description of the script
  --node <nodeId>            Target cluster node ID (default: local or *)
  --namespace <ns>           Script namespace (default: script)
  --disabled                 Create in disabled state (default: enabled)

Test Sandbox Options:
  --topic <topic>            Simulated incoming MQTT message topic
  --payload <payload>        Simulated MQTT payload (JSON object or string)
  --args <json>              Arguments JSON string for CALLABLE scripts

Skill Export Options:
  --output <file>, -o <file> Save AI skill directly to a file
  --install [dir]            Install as SKILL.md into local skills directory
`

func runScriptCommand(ctx context.Context, client *Client, args []string) error {
	if len(args) == 0 {
		return runScriptList(ctx, client, nil)
	}

	cmd := strings.ToLower(args[0])
	subargs := args[1:]

	switch cmd {
	case "-h", "--help", "help":
		fmt.Print(scriptHelp)
		return nil
	case "list", "ls":
		return runScriptList(ctx, client, subargs)
	case "get", "inspect", "show":
		return runScriptGet(ctx, client, subargs)
	case "create", "new", "add":
		return runScriptCreate(ctx, client, subargs)
	case "update", "edit":
		return runScriptUpdate(ctx, client, subargs)
	case "delete", "rm", "remove":
		return runScriptDelete(ctx, client, subargs)
	case "toggle":
		return runScriptToggle(ctx, client, subargs)
	case "enable":
		if len(subargs) == 0 {
			return fmt.Errorf("usage: mmq script enable <name>")
		}
		return runScriptToggle(ctx, client, []string{subargs[0], "on"})
	case "disable":
		if len(subargs) == 0 {
			return fmt.Errorf("usage: mmq script disable <name>")
		}
		return runScriptToggle(ctx, client, []string{subargs[0], "off"})
	case "start":
		if len(subargs) == 0 {
			return fmt.Errorf("usage: mmq script start <name>")
		}
		return runScriptToggle(ctx, client, []string{subargs[0], "on"})
	case "stop":
		if len(subargs) == 0 {
			return fmt.Errorf("usage: mmq script stop <name>")
		}
		return runScriptToggle(ctx, client, []string{subargs[0], "off"})
	case "test", "run", "dry-run":
		return runScriptTest(ctx, client, subargs)
	case "logs", "log":
		return runScriptLogs(ctx, client, subargs)
	case "docs", "doc":
		return runScriptDocs(ctx, client, subargs)
	case "skill", "skills":
		return runScriptSkill(ctx, client, subargs)
	case "languages", "langs":
		return runScriptLanguages(ctx, client, subargs)
	default:
		// If arg doesn't start with flag, treat as script get or list filter
		if !strings.HasPrefix(cmd, "-") {
			return runScriptGet(ctx, client, args)
		}
		return fmt.Errorf("unknown script command %q. Run 'mmq script --help' for usage", cmd)
	}
}

// ----------------------------------------------------------------------------
// 1. Script List
// ----------------------------------------------------------------------------

func runScriptList(ctx context.Context, client *Client, args []string) error {
	var nameFilter, nodeFilter string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case (arg == "--node" || arg == "-n") && i+1 < len(args):
			i++
			nodeFilter = args[i]
		case !strings.HasPrefix(arg, "-"):
			nameFilter = arg
		}
	}

	query := `
		query ListScripts($name: String, $nodeId: String) {
			scripts(name: $name, nodeId: $nodeId) {
				name
				namespace
				nodeId
				enabled
				createdAt
				updatedAt
				executionCount
				errorCount
				lastExecutionTime
				lastExecutionStatus
				config {
					language
					triggerType
					topicFilters
					timerIntervalMs
					instanceMode
					description
				}
			}
		}
	`
	vars := map[string]any{}
	if nameFilter != "" {
		vars["name"] = nameFilter
	}
	if nodeFilter != "" {
		vars["nodeId"] = nodeFilter
	}

	var res struct {
		Data struct {
			Scripts []ScriptData `json:"scripts"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}

	if err := client.DoQuery(ctx, query, vars, &res); err != nil {
		return err
	}
	if len(res.Errors) > 0 {
		return fmt.Errorf("%s", res.Errors[0].Message)
	}

	if client.cfg.JSONMode {
		out, _ := json.MarshalIndent(res.Data.Scripts, "", "  ")
		fmt.Println(string(out))
		return nil
	}

	if len(res.Data.Scripts) == 0 {
		fmt.Println("No broker scripts found.")
		return nil
	}

	fmt.Printf("%-24s %-10s %-9s %-8s %-12s %-12s %-10s %s\n",
		"NAME", "LANGUAGE", "TRIGGER", "STATUS", "EXECUTIONS", "ERRORS", "LAST TIME", "DESCRIPTION")
	fmt.Println(strings.Repeat("-", 100))

	for _, s := range res.Data.Scripts {
		status := "Enabled"
		if !s.Enabled {
			status = "Disabled"
		}
		desc := ""
		if s.Config.Description != nil {
			desc = *s.Config.Description
			if len(desc) > 30 {
				desc = desc[:27] + "..."
			}
		}

		lastTime := "-"
		if s.LastExecutionTime != nil && *s.LastExecutionTime != "" {
			parts := strings.Split(*s.LastExecutionTime, "T")
			if len(parts) == 2 {
				timePart := strings.TrimSuffix(parts[1], "Z")
				if len(timePart) > 8 {
					timePart = timePart[:8]
				}
				lastTime = timePart
			} else {
				lastTime = *s.LastExecutionTime
			}
		}

		trigger := s.Config.TriggerType
		if trigger == "TIMER" && s.Config.TimerIntervalMs > 0 {
			trigger = fmt.Sprintf("%dms", s.Config.TimerIntervalMs)
		} else if trigger == "TOPIC" && len(s.Config.TopicFilters) > 0 {
			trigger = s.Config.TopicFilters[0]
			if len(s.Config.TopicFilters) > 1 {
				trigger = fmt.Sprintf("%s (+%d)", trigger, len(s.Config.TopicFilters)-1)
			}
		}

		fmt.Printf("%-24s %-10s %-9s %-8s %-12d %-12d %-10s %s\n",
			s.Name,
			s.Config.Language,
			trigger,
			status,
			s.ExecutionCount,
			s.ErrorCount,
			lastTime,
			desc,
		)
	}

	return nil
}

// ----------------------------------------------------------------------------
// 2. Script Get
// ----------------------------------------------------------------------------

func runScriptGet(ctx context.Context, client *Client, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: mmq script get <name>")
	}
	name := args[0]

	query := `
		query GetScript($name: String!) {
			script(name: $name) {
				name
				namespace
				nodeId
				enabled
				createdAt
				updatedAt
				isOnCurrentNode
				executionCount
				errorCount
				lastExecutionTime
				lastExecutionStatus
				recentLogs
				config {
					language
					script
					triggerType
					topicFilters
					triggerOnChangeOnly
					timerIntervalMs
					instanceMode
					timeoutMs
					description
				}
			}
		}
	`

	var res struct {
		Data struct {
			Script *ScriptData `json:"script"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}

	if err := client.DoQuery(ctx, query, map[string]any{"name": name}, &res); err != nil {
		return err
	}
	if len(res.Errors) > 0 {
		return fmt.Errorf("%s", res.Errors[0].Message)
	}
	if res.Data.Script == nil {
		return fmt.Errorf("script %q not found", name)
	}

	s := res.Data.Script
	if client.cfg.JSONMode {
		out, _ := json.MarshalIndent(s, "", "  ")
		fmt.Println(string(out))
		return nil
	}

	fmt.Printf("Script: %s\n", s.Name)
	fmt.Printf("  Namespace:        %s\n", s.Namespace)
	fmt.Printf("  Node ID:          %s\n", s.NodeID)
	fmt.Printf("  Enabled:          %v\n", s.Enabled)
	fmt.Printf("  Language:         %s\n", s.Config.Language)
	fmt.Printf("  Trigger Type:     %s\n", s.Config.TriggerType)
	if len(s.Config.TopicFilters) > 0 {
		fmt.Printf("  Topic Filters:    %s\n", strings.Join(s.Config.TopicFilters, ", "))
		fmt.Printf("  On Change Only:   %v\n", s.Config.TriggerOnChangeOnly)
	}
	if s.Config.TimerIntervalMs > 0 {
		fmt.Printf("  Timer Interval:   %d ms\n", s.Config.TimerIntervalMs)
	}
	fmt.Printf("  Instance Mode:    %s\n", s.Config.InstanceMode)
	fmt.Printf("  Timeout:          %d ms\n", s.Config.TimeoutMs)
	if s.Config.Description != nil && *s.Config.Description != "" {
		fmt.Printf("  Description:      %s\n", *s.Config.Description)
	}
	fmt.Printf("  Created:          %s\n", s.CreatedAt)
	fmt.Printf("  Updated:          %s\n", s.UpdatedAt)
	fmt.Printf("  Executions:       %d\n", s.ExecutionCount)
	fmt.Printf("  Errors:           %d\n", s.ErrorCount)
	if s.LastExecutionTime != nil {
		fmt.Printf("  Last Run:         %s (%s)\n", *s.LastExecutionTime, derefStr(s.LastExecutionStatus))
	}

	fmt.Println("\nScript Code:")
	fmt.Println(strings.Repeat("-", 60))
	fmt.Println(s.Config.Script)
	fmt.Println(strings.Repeat("-", 60))

	if len(s.RecentLogs) > 0 {
		fmt.Println("\nRecent Logs:")
		for _, log := range s.RecentLogs {
			fmt.Printf("  %s\n", log)
		}
	}

	return nil
}

// ----------------------------------------------------------------------------
// 3. Script Create
// ----------------------------------------------------------------------------

func runScriptCreate(ctx context.Context, client *Client, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: mmq script create <name> [options]")
	}

	var name string
	var lang, triggerType, mode, desc, nodeID, namespace, scriptCode, filePath string
	var timerInterval, timeout int
	var topicFilters []string
	var triggerOnChange bool
	enabled := true

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case (arg == "--lang" || arg == "-l") && i+1 < len(args):
			i++
			lang = args[i]
		case (arg == "--trigger" || arg == "-t") && i+1 < len(args):
			i++
			triggerType = strings.ToUpper(args[i])
		case (arg == "--topic") && i+1 < len(args):
			i++
			for _, tf := range strings.Split(args[i], ",") {
				tf = strings.TrimSpace(tf)
				if tf != "" {
					topicFilters = append(topicFilters, tf)
				}
			}
		case (arg == "--interval") && i+1 < len(args):
			i++
			timerInterval, _ = strconv.Atoi(args[i])
		case arg == "--on-change":
			triggerOnChange = true
		case arg == "--mode" && i+1 < len(args):
			i++
			mode = strings.ToUpper(args[i])
		case arg == "--timeout" && i+1 < len(args):
			i++
			timeout, _ = strconv.Atoi(args[i])
		case arg == "--code" && i+1 < len(args):
			i++
			scriptCode = args[i]
		case (arg == "--file" || arg == "-f") && i+1 < len(args):
			i++
			filePath = args[i]
		case (arg == "--desc" || arg == "-d") && i+1 < len(args):
			i++
			desc = args[i]
		case arg == "--node" && i+1 < len(args):
			i++
			nodeID = args[i]
		case arg == "--namespace" && i+1 < len(args):
			i++
			namespace = args[i]
		case arg == "--disabled":
			enabled = false
		case !strings.HasPrefix(arg, "-") && name == "":
			name = arg
		}
	}

	if name == "" {
		return fmt.Errorf("script name is required")
	}

	if filePath != "" {
		content, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("failed to read script file %q: %w", filePath, err)
		}
		scriptCode = string(content)
	}

	if strings.TrimSpace(scriptCode) == "" {
		return fmt.Errorf("script code is required (use --code or --file)")
	}

	if lang == "" {
		lang = "starlark"
	}
	if triggerType == "" {
		if timerInterval > 0 && len(topicFilters) > 0 {
			triggerType = "BOTH"
		} else if timerInterval > 0 {
			triggerType = "TIMER"
		} else {
			triggerType = "TOPIC"
		}
	}
	if mode == "" {
		mode = "SINGLETON"
	}
	if timeout <= 0 {
		timeout = 200
	}
	if namespace == "" {
		namespace = "script"
	}
	if nodeID == "" {
		nodeID = "local"
	}

	input := ScriptInput{
		Name:      name,
		Namespace: namespace,
		NodeID:    nodeID,
		Enabled:   enabled,
		Config: ScriptConfigInput{
			Language:            lang,
			Script:              scriptCode,
			TriggerType:         triggerType,
			TopicFilters:        topicFilters,
			TriggerOnChangeOnly: triggerOnChange,
			TimerIntervalMs:     timerInterval,
			InstanceMode:        mode,
			TimeoutMs:           timeout,
			Description:         desc,
		},
	}

	mutation := `
		mutation CreateScript($input: ScriptInput!) {
			script {
				create(input: $input) {
					success
					errors
					script {
						name
						enabled
						config {
							language
							triggerType
						}
					}
				}
			}
		}
	`

	var res struct {
		Data struct {
			Script struct {
				Create struct {
					Success bool     `json:"success"`
					Errors  []string `json:"errors"`
				} `json:"create"`
			} `json:"script"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}

	if err := client.DoQuery(ctx, mutation, map[string]any{"input": input}, &res); err != nil {
		return err
	}
	if len(res.Errors) > 0 {
		return fmt.Errorf("%s", res.Errors[0].Message)
	}
	if !res.Data.Script.Create.Success {
		return fmt.Errorf("create failed: %s", strings.Join(res.Data.Script.Create.Errors, ", "))
	}

	if client.cfg.JSONMode {
		fmt.Printf(`{"success": true, "name": %q}`+"\n", name)
	} else {
		fmt.Printf("✓ Script %q created successfully (%s, %s)\n", name, lang, triggerType)
	}
	return nil
}

// ----------------------------------------------------------------------------
// 4. Script Update
// ----------------------------------------------------------------------------

func runScriptUpdate(ctx context.Context, client *Client, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: mmq script update <name> [options]")
	}
	name := args[0]

	// Fetch existing script first
	getQuery := `
		query GetScript($name: String!) {
			script(name: $name) {
				name
				namespace
				nodeId
				enabled
				config {
					language
					script
					triggerType
					topicFilters
					triggerOnChangeOnly
					timerIntervalMs
					instanceMode
					timeoutMs
					description
				}
			}
		}
	`
	var getRes struct {
		Data struct {
			Script *ScriptData `json:"script"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := client.DoQuery(ctx, getQuery, map[string]any{"name": name}, &getRes); err != nil {
		return err
	}
	if getRes.Data.Script == nil {
		return fmt.Errorf("script %q not found", name)
	}
	existing := getRes.Data.Script

	// Parse flags and overlay
	subargs := args[1:]
	for i := 0; i < len(subargs); i++ {
		arg := subargs[i]
		switch {
		case (arg == "--lang" || arg == "-l") && i+1 < len(subargs):
			i++
			existing.Config.Language = subargs[i]
		case (arg == "--trigger" || arg == "-t") && i+1 < len(subargs):
			i++
			existing.Config.TriggerType = strings.ToUpper(subargs[i])
		case (arg == "--topic") && i+1 < len(subargs):
			i++
			var tfs []string
			for _, tf := range strings.Split(subargs[i], ",") {
				if t := strings.TrimSpace(tf); t != "" {
					tfs = append(tfs, t)
				}
			}
			existing.Config.TopicFilters = tfs
		case (arg == "--interval") && i+1 < len(subargs):
			i++
			existing.Config.TimerIntervalMs, _ = strconv.Atoi(subargs[i])
		case arg == "--on-change":
			existing.Config.TriggerOnChangeOnly = true
		case arg == "--no-on-change":
			existing.Config.TriggerOnChangeOnly = false
		case arg == "--mode" && i+1 < len(subargs):
			i++
			existing.Config.InstanceMode = strings.ToUpper(subargs[i])
		case arg == "--timeout" && i+1 < len(subargs):
			i++
			existing.Config.TimeoutMs, _ = strconv.Atoi(subargs[i])
		case arg == "--code" && i+1 < len(subargs):
			i++
			existing.Config.Script = subargs[i]
		case (arg == "--file" || arg == "-f") && i+1 < len(subargs):
			i++
			content, err := os.ReadFile(subargs[i])
			if err != nil {
				return fmt.Errorf("failed to read file %q: %w", subargs[i], err)
			}
			existing.Config.Script = string(content)
		case (arg == "--desc" || arg == "-d") && i+1 < len(subargs):
			i++
			desc := subargs[i]
			existing.Config.Description = &desc
		case arg == "--node" && i+1 < len(subargs):
			i++
			existing.NodeID = subargs[i]
		case arg == "--namespace" && i+1 < len(subargs):
			i++
			existing.Namespace = subargs[i]
		case arg == "--enabled":
			existing.Enabled = true
		case arg == "--disabled":
			existing.Enabled = false
		}
	}

	descStr := ""
	if existing.Config.Description != nil {
		descStr = *existing.Config.Description
	}

	input := ScriptInput{
		Name:      existing.Name,
		Namespace: existing.Namespace,
		NodeID:    existing.NodeID,
		Enabled:   existing.Enabled,
		Config: ScriptConfigInput{
			Language:            existing.Config.Language,
			Script:              existing.Config.Script,
			TriggerType:         existing.Config.TriggerType,
			TopicFilters:        existing.Config.TopicFilters,
			TriggerOnChangeOnly: existing.Config.TriggerOnChangeOnly,
			TimerIntervalMs:     existing.Config.TimerIntervalMs,
			InstanceMode:        existing.Config.InstanceMode,
			TimeoutMs:           existing.Config.TimeoutMs,
			Description:         descStr,
		},
	}

	mutation := `
		mutation UpdateScript($name: String!, $input: ScriptInput!) {
			script {
				update(name: $name, input: $input) {
					success
					errors
				}
			}
		}
	`

	var updateRes struct {
		Data struct {
			Script struct {
				Update struct {
					Success bool     `json:"success"`
					Errors  []string `json:"errors"`
				} `json:"update"`
			} `json:"script"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}

	if err := client.DoQuery(ctx, mutation, map[string]any{"name": name, "input": input}, &updateRes); err != nil {
		return err
	}
	if len(updateRes.Errors) > 0 {
		return fmt.Errorf("%s", updateRes.Errors[0].Message)
	}
	if !updateRes.Data.Script.Update.Success {
		return fmt.Errorf("update failed: %s", strings.Join(updateRes.Data.Script.Update.Errors, ", "))
	}

	if client.cfg.JSONMode {
		fmt.Printf(`{"success": true, "name": %q}`+"\n", name)
	} else {
		fmt.Printf("✓ Script %q updated successfully\n", name)
	}
	return nil
}

// ----------------------------------------------------------------------------
// 5. Script Delete
// ----------------------------------------------------------------------------

func runScriptDelete(ctx context.Context, client *Client, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: mmq script delete <name1> [name2...]")
	}

	mutation := `
		mutation DeleteScript($name: String!) {
			script {
				delete(name: $name)
			}
		}
	`

	for _, name := range args {
		var res struct {
			Data struct {
				Script struct {
					Delete bool `json:"delete"`
				} `json:"script"`
			} `json:"data"`
			Errors []struct {
				Message string `json:"message"`
			} `json:"errors"`
		}

		if err := client.DoQuery(ctx, mutation, map[string]any{"name": name}, &res); err != nil {
			return fmt.Errorf("failed to delete %q: %w", name, err)
		}
		if len(res.Errors) > 0 {
			return fmt.Errorf("failed to delete %q: %s", name, res.Errors[0].Message)
		}
		if !res.Data.Script.Delete {
			return fmt.Errorf("script %q could not be deleted (not found)", name)
		}

		if client.cfg.JSONMode {
			fmt.Printf(`{"success": true, "deleted": %q}`+"\n", name)
		} else {
			fmt.Printf("✓ Deleted script %q\n", name)
		}
	}
	return nil
}

// ----------------------------------------------------------------------------
// 6. Script Toggle
// ----------------------------------------------------------------------------

func runScriptToggle(ctx context.Context, client *Client, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: mmq script toggle <name> <on|off|true|false>")
	}
	name := args[0]
	val := strings.ToLower(args[1])
	enabled := val == "on" || val == "true" || val == "1" || val == "enable"

	mutation := `
		mutation ToggleScript($name: String!, $enabled: Boolean!) {
			script {
				toggle(name: $name, enabled: $enabled) {
					success
					errors
					script {
						name
						enabled
					}
				}
			}
		}
	`

	var res struct {
		Data struct {
			Script struct {
				Toggle struct {
					Success bool     `json:"success"`
					Errors  []string `json:"errors"`
				} `json:"toggle"`
			} `json:"script"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}

	if err := client.DoQuery(ctx, mutation, map[string]any{"name": name, "enabled": enabled}, &res); err != nil {
		return err
	}
	if len(res.Errors) > 0 {
		return fmt.Errorf("%s", res.Errors[0].Message)
	}
	if !res.Data.Script.Toggle.Success {
		return fmt.Errorf("toggle failed: %s", strings.Join(res.Data.Script.Toggle.Errors, ", "))
	}

	state := "enabled"
	if !enabled {
		state = "disabled"
	}
	if client.cfg.JSONMode {
		fmt.Printf(`{"success": true, "name": %q, "enabled": %v}`+"\n", name, enabled)
	} else {
		fmt.Printf("✓ Script %q is now %s\n", name, state)
	}
	return nil
}

// ----------------------------------------------------------------------------
// 7. Script Test Sandbox
// ----------------------------------------------------------------------------

func runScriptTest(ctx context.Context, client *Client, args []string) error {
	var targetName, testTopic, testPayload, testArgs string
	var code, filePath, lang string

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case (arg == "--topic" || arg == "-t") && i+1 < len(args):
			i++
			testTopic = args[i]
		case (arg == "--payload" || arg == "-p") && i+1 < len(args):
			i++
			testPayload = args[i]
		case (arg == "--args" || arg == "-a") && i+1 < len(args):
			i++
			testArgs = args[i]
		case arg == "--code" && i+1 < len(args):
			i++
			code = args[i]
		case (arg == "--file" || arg == "-f") && i+1 < len(args):
			i++
			filePath = args[i]
		case (arg == "--lang" || arg == "-l") && i+1 < len(args):
			i++
			lang = args[i]
		case !strings.HasPrefix(arg, "-") && targetName == "":
			targetName = arg
		}
	}

	if filePath != "" {
		content, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("failed to read test file %q: %w", filePath, err)
		}
		code = string(content)
	}

	var input ScriptInput
	if code != "" {
		if lang == "" {
			lang = "starlark"
		}
		name := targetName
		if name == "" {
			name = "SandboxDryRun"
		}
		input = ScriptInput{
			Name:      name,
			Namespace: "script",
			NodeID:    "local",
			Enabled:   true,
			Config: ScriptConfigInput{
				Language:     lang,
				Script:       code,
				TriggerType:  "CALLABLE",
				InstanceMode: "SINGLETON",
				TimeoutMs:    500,
			},
		}
	} else if targetName != "" {
		// Fetch existing script config
		getQuery := `
			query GetScript($name: String!) {
				script(name: $name) {
					name
					namespace
					nodeId
					enabled
					config {
						language
						script
						triggerType
						topicFilters
						triggerOnChangeOnly
						timerIntervalMs
						instanceMode
						timeoutMs
						description
					}
				}
			}
		`
		var getRes struct {
			Data struct {
				Script *ScriptData `json:"script"`
			} `json:"data"`
			Errors []struct {
				Message string `json:"message"`
			} `json:"errors"`
		}
		if err := client.DoQuery(ctx, getQuery, map[string]any{"name": targetName}, &getRes); err != nil {
			return err
		}
		if getRes.Data.Script == nil {
			return fmt.Errorf("script %q not found", targetName)
		}
		s := getRes.Data.Script
		input = ScriptInput{
			Name:      s.Name,
			Namespace: s.Namespace,
			NodeID:    s.NodeID,
			Enabled:   s.Enabled,
			Config: ScriptConfigInput{
				Language:            s.Config.Language,
				Script:              s.Config.Script,
				TriggerType:         s.Config.TriggerType,
				TopicFilters:        s.Config.TopicFilters,
				TriggerOnChangeOnly: s.Config.TriggerOnChangeOnly,
				TimerIntervalMs:     s.Config.TimerIntervalMs,
				InstanceMode:        s.Config.InstanceMode,
				TimeoutMs:           s.Config.TimeoutMs,
			},
		}
	} else {
		return fmt.Errorf("usage: mmq script test <name|--code <code>> [options]")
	}

	mutation := `
		mutation TestScript($input: ScriptInput!, $testTopic: String, $testPayload: String, $testArgs: String) {
			script {
				test(input: $input, testTopic: $testTopic, testPayload: $testPayload, testArgs: $testArgs) {
					success
					returnValue
					executionTimeMs
					outputMessages {
						topic
						payload
						qos
						retain
					}
					logs
					errors
				}
			}
		}
	`

	vars := map[string]any{
		"input": input,
	}
	if testTopic != "" {
		vars["testTopic"] = testTopic
	}
	if testPayload != "" {
		vars["testPayload"] = testPayload
	}
	if testArgs != "" {
		vars["testArgs"] = testArgs
	}

	var res struct {
		Data struct {
			Script struct {
				Test ScriptTestResult `json:"test"`
			} `json:"script"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}

	if err := client.DoQuery(ctx, mutation, vars, &res); err != nil {
		return err
	}
	if len(res.Errors) > 0 {
		return fmt.Errorf("%s", res.Errors[0].Message)
	}

	testRes := res.Data.Script.Test
	if client.cfg.JSONMode {
		out, _ := json.MarshalIndent(testRes, "", "  ")
		fmt.Println(string(out))
		return nil
	}

	statusStr := "SUCCESS"
	if !testRes.Success {
		statusStr = "FAILED"
	}

	fmt.Printf("Sandbox Dry-Run Result: [%s] (%.2f ms)\n", statusStr, testRes.ExecutionTimeMs)

	if testRes.ReturnValue != nil && *testRes.ReturnValue != "" {
		fmt.Printf("Return Value: %s\n", *testRes.ReturnValue)
	}

	if len(testRes.OutputMessages) > 0 {
		fmt.Printf("\nPublished Messages (%d):\n", len(testRes.OutputMessages))
		for _, msg := range testRes.OutputMessages {
			retainFlag := ""
			if msg.Retain {
				retainFlag = " [RETAIN]"
			}
			fmt.Printf("  • %s (QoS %d%s): %s\n", msg.Topic, msg.QoS, retainFlag, msg.Payload)
		}
	}

	if len(testRes.Logs) > 0 {
		fmt.Printf("\nCaptured Logs (%d):\n", len(testRes.Logs))
		for _, log := range testRes.Logs {
			fmt.Printf("  %s\n", log)
		}
	}

	if len(testRes.Errors) > 0 {
		fmt.Printf("\nErrors (%d):\n", len(testRes.Errors))
		for _, errStr := range testRes.Errors {
			fmt.Printf("  ✗ %s\n", errStr)
		}
		return fmt.Errorf("script execution failed with %d error(s)", len(testRes.Errors))
	}

	return nil
}

// ----------------------------------------------------------------------------
// 8. Script Logs
// ----------------------------------------------------------------------------

func runScriptLogs(ctx context.Context, client *Client, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: mmq script logs <name>")
	}
	name := args[0]

	query := `
		query GetScriptLogs($name: String!) {
			script(name: $name) {
				name
				recentLogs
			}
		}
	`

	var res struct {
		Data struct {
			Script *struct {
				Name       string   `json:"name"`
				RecentLogs []string `json:"recentLogs"`
			} `json:"script"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}

	if err := client.DoQuery(ctx, query, map[string]any{"name": name}, &res); err != nil {
		return err
	}
	if len(res.Errors) > 0 {
		return fmt.Errorf("%s", res.Errors[0].Message)
	}
	if res.Data.Script == nil {
		return fmt.Errorf("script %q not found", name)
	}

	if client.cfg.JSONMode {
		out, _ := json.MarshalIndent(res.Data.Script.RecentLogs, "", "  ")
		fmt.Println(string(out))
		return nil
	}

	if len(res.Data.Script.RecentLogs) == 0 {
		fmt.Printf("No recent execution logs for script %q.\n", name)
		return nil
	}

	fmt.Printf("Execution logs for %q (%d entries):\n", name, len(res.Data.Script.RecentLogs))
	for _, l := range res.Data.Script.RecentLogs {
		fmt.Printf("  %s\n", l)
	}

	return nil
}

// ----------------------------------------------------------------------------
// 9. Script Languages
// ----------------------------------------------------------------------------

func runScriptLanguages(ctx context.Context, client *Client, _ []string) error {
	query := `
		query GetScriptLanguages {
			scriptLanguages {
				name
				displayName
				description
				isDefault
			}
		}
	`

	var res struct {
		Data struct {
			ScriptLanguages []ScriptLanguageData `json:"scriptLanguages"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}

	if err := client.DoQuery(ctx, query, nil, &res); err != nil {
		return err
	}
	if len(res.Errors) > 0 {
		return fmt.Errorf("%s", res.Errors[0].Message)
	}

	if client.cfg.JSONMode {
		out, _ := json.MarshalIndent(res.Data.ScriptLanguages, "", "  ")
		fmt.Println(string(out))
		return nil
	}

	fmt.Printf("%-14s %-32s %s\n", "LANGUAGE", "DISPLAY NAME", "DESCRIPTION")
	fmt.Println(strings.Repeat("-", 80))
	for _, l := range res.Data.ScriptLanguages {
		def := ""
		if l.IsDefault != nil && *l.IsDefault {
			def = " (Default)"
		}
		desc := ""
		if l.Description != nil {
			desc = *l.Description
		}
		fmt.Printf("%-14s %-32s %s\n", l.Name+def, l.DisplayName, desc)
	}

	return nil
}

// ----------------------------------------------------------------------------
// 10. Script Docs
// ----------------------------------------------------------------------------

func runScriptDocs(ctx context.Context, client *Client, args []string) error {
	var lang string
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		lang = args[0]
	}

	// Try dedicated query first
	docQuery := `
		query GetScriptDoc($language: String) {
			scriptDocumentation(language: $language)
		}
	`
	var res struct {
		Data struct {
			ScriptDocumentation string `json:"scriptDocumentation"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}

	err := client.DoQuery(ctx, docQuery, map[string]any{"language": lang}, &res)
	if err == nil && len(res.Errors) == 0 && res.Data.ScriptDocumentation != "" {
		fmt.Println(res.Data.ScriptDocumentation)
		return nil
	}

	// Fallback to scriptLanguages
	langQuery := `
		query GetScriptLanguages {
			scriptLanguages {
				name
				displayName
				documentation
			}
		}
	`
	var langRes struct {
		Data struct {
			ScriptLanguages []ScriptLanguageData `json:"scriptLanguages"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}

	if err2 := client.DoQuery(ctx, langQuery, nil, &langRes); err2 != nil {
		return err2
	}
	if len(langRes.Errors) > 0 {
		return fmt.Errorf("%s", langRes.Errors[0].Message)
	}

	for _, l := range langRes.Data.ScriptLanguages {
		if lang == "" || strings.EqualFold(l.Name, lang) {
			if l.Documentation != "" {
				fmt.Println(l.Documentation)
				return nil
			}
		}
	}

	return fmt.Errorf("no documentation found for language %q", lang)
}

// ----------------------------------------------------------------------------
// 11. Script Skill Export & Install
// ----------------------------------------------------------------------------

func runScriptSkill(ctx context.Context, client *Client, args []string) error {
	var lang, outputFile, installDir string
	installFlag := false

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case (arg == "--output" || arg == "-o") && i+1 < len(args):
			i++
			outputFile = args[i]
		case arg == "--install":
			installFlag = true
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				i++
				installDir = args[i]
			}
		case !strings.HasPrefix(arg, "-") && lang == "":
			lang = arg
		}
	}

	// Try dedicated query first
	skillQuery := `
		query GetScriptSkill($language: String) {
			scriptSkill(language: $language)
		}
	`
	var res struct {
		Data struct {
			ScriptSkill string `json:"scriptSkill"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}

	var skillContent string
	err := client.DoQuery(ctx, skillQuery, map[string]any{"language": lang}, &res)
	if err == nil && len(res.Errors) == 0 && res.Data.ScriptSkill != "" {
		skillContent = res.Data.ScriptSkill
	} else {
		// Fallback to scriptLanguages
		langQuery := `
			query GetScriptLanguages {
				scriptLanguages {
					name
					displayName
					skill
				}
			}
		`
		var langRes struct {
			Data struct {
				ScriptLanguages []ScriptLanguageData `json:"scriptLanguages"`
			} `json:"data"`
			Errors []struct {
				Message string `json:"message"`
			} `json:"errors"`
		}

		if err2 := client.DoQuery(ctx, langQuery, nil, &langRes); err2 != nil {
			return err2
		}
		if len(langRes.Errors) > 0 {
			return fmt.Errorf("%s", langRes.Errors[0].Message)
		}

		for _, l := range langRes.Data.ScriptLanguages {
			if lang == "" || strings.EqualFold(l.Name, lang) {
				if l.Skill != "" {
					skillContent = l.Skill
					break
				}
			}
		}
	}

	if skillContent == "" {
		return fmt.Errorf("no skill instructions found for language %q", lang)
	}

	// If install requested
	if installFlag {
		if installDir == "" {
			homeDir, err := os.UserHomeDir()
			if err == nil {
				installDir = filepath.Join(homeDir, ".gemini", "antigravity", "skills", "monstermq-scripts")
			} else {
				installDir = filepath.Join(".", ".agents", "skills", "monstermq-scripts")
			}
		}
		if err := os.MkdirAll(installDir, 0755); err != nil {
			return fmt.Errorf("failed to create skill directory %q: %w", installDir, err)
		}
		targetPath := filepath.Join(installDir, "SKILL.md")
		if err := os.WriteFile(targetPath, []byte(skillContent), 0644); err != nil {
			return fmt.Errorf("failed to write skill file %q: %w", targetPath, err)
		}
		fmt.Printf("✓ Installed AI skill to: %s\n", targetPath)
		return nil
	}

	// If output file requested
	if outputFile != "" {
		if err := os.WriteFile(outputFile, []byte(skillContent), 0644); err != nil {
			return fmt.Errorf("failed to write skill to %q: %w", outputFile, err)
		}
		fmt.Printf("✓ Saved AI skill to: %s\n", outputFile)
		return nil
	}

	// Otherwise output to stdout
	fmt.Print(skillContent)
	return nil
}

func derefStr(s *string) string {
	if s == nil {
		return "-"
	}
	return *s
}
