package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// formatPayload decodes base64 string payloads into readable UTF-8 text when valid.
func formatPayload(payload string) string {
	if payload == "" {
		return ""
	}
	decoded, err := base64.StdEncoding.DecodeString(payload)
	if err == nil && len(decoded) > 0 {
		if utf8.Valid(decoded) && isPrintable(string(decoded)) {
			return string(decoded)
		}
	}
	return payload
}

func isPrintable(s string) bool {
	for _, r := range s {
		if r < 32 && r != '\t' && r != '\n' && r != '\r' {
			return false
		}
	}
	return true
}

// formatTimestampISO formats epoch timestamps (seconds, millis, micros, nanos) as ISO 8601 UTC.
func formatTimestampISO(ts int64) string {
	if ts <= 0 {
		return ""
	}
	var t time.Time
	if ts > 1e18 { // nanoseconds
		t = time.Unix(0, ts).UTC()
	} else if ts > 1e15 { // microseconds
		t = time.UnixMicro(ts).UTC()
	} else if ts > 1e11 { // milliseconds (e.g. 1786683732069)
		t = time.UnixMilli(ts).UTC()
	} else { // seconds
		t = time.Unix(ts, 0).UTC()
	}
	return t.Format(time.RFC3339)
}

// formatPayloadAndType decodes base64 string payloads and infers if it is text/JSON.
func formatPayloadAndType(payload string, origFormat string) (string, string) {
	if payload == "" {
		return "", origFormat
	}
	decoded, err := base64.StdEncoding.DecodeString(payload)
	if err == nil && len(decoded) > 0 {
		if utf8.Valid(decoded) && isPrintable(string(decoded)) {
			detectedType := "STRING"
			trimmed := strings.TrimSpace(string(decoded))
			if (strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}")) ||
				(strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]")) {
				var js json.RawMessage
				if json.Unmarshal([]byte(trimmed), &js) == nil {
					detectedType = "JSON"
				}
			}
			return string(decoded), detectedType
		}
	}
	return payload, origFormat
}

func hasHelpFlag(args []string) bool {
	for _, a := range args {
		if a == "-h" || a == "--help" || a == "help" {
			return true
		}
	}
	return false
}

func runGetValue(ctx context.Context, client *Client, args []string) error {
	if len(args) < 1 || hasHelpFlag(args) {
		fmt.Println("Usage: mmq currentValue <topic> [archiveGroup] [options]")
		fmt.Println()
		fmt.Println("Fetch current or retained payload and metadata for a specific topic.")
		fmt.Println()
		fmt.Println("Arguments:")
		fmt.Println("  <topic>             Topic name (required)")
		fmt.Println("  [archiveGroup]      Archive group name (default: Default)")
		fmt.Println()
		fmt.Println("Options:")
		fmt.Println("  --archive-group, -g Archive group name (alternative to positional argument)")
		fmt.Println("  -h, --help          Show this help text")
		return nil
	}

	var topic string
	archiveGroup := "Default"

	var nonFlagArgs []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "-") {
			switch arg {
			case "--archive-group", "--group", "-g":
				if i+1 < len(args) {
					archiveGroup = args[i+1]
					i++
				}
			}
		} else {
			nonFlagArgs = append(nonFlagArgs, arg)
		}
	}

	if len(nonFlagArgs) > 0 {
		topic = nonFlagArgs[0]
	}
	if len(nonFlagArgs) > 1 {
		archiveGroup = nonFlagArgs[1]
	}

	if topic == "" {
		fmt.Println("Usage: mmq currentValue <topic> [archiveGroup] [options]")
		return nil
	}

	query := `
		query GetValue($topic: String!, $archiveGroup: String!) {
			currentValue(topic: $topic, archiveGroup: $archiveGroup) {
				topic
				payload
				format
				timestamp
				qos
			}
			retainedMessage(topic: $topic) {
				topic
				payload
				format
				timestamp
				qos
			}
		}
	`
	var res struct {
		Data struct {
			CurrentValue *struct {
				Topic     string `json:"topic"`
				Payload   string `json:"payload"`
				Format    string `json:"format"`
				Timestamp int64  `json:"timestamp"`
				QoS       int    `json:"qos"`
			} `json:"currentValue"`
			RetainedMessage *struct {
				Topic     string `json:"topic"`
				Payload   string `json:"payload"`
				Format    string `json:"format"`
				Timestamp int64  `json:"timestamp"`
				QoS       int    `json:"qos"`
			} `json:"retainedMessage"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}

	if err := client.DoQuery(ctx, query, map[string]any{
		"topic":        topic,
		"archiveGroup": archiveGroup,
	}, &res); err != nil {
		return err
	}
	if len(res.Errors) > 0 {
		return fmt.Errorf("GraphQL error: %s", res.Errors[0].Message)
	}

	if client.cfg.JSONMode {
		return printJSON(res.Data)
	}

	val := res.Data.CurrentValue
	if val == nil {
		val = res.Data.RetainedMessage
	}
	if val == nil {
		fmt.Printf("No value found for topic '%s'\n", topic)
		return nil
	}

	isoTime := formatTimestampISO(val.Timestamp)
	timeStr := strconv.FormatInt(val.Timestamp, 10)
	if isoTime != "" {
		timeStr = isoTime
	}

	payloadStr, formatStr := formatPayloadAndType(val.Payload, val.Format)

	fmt.Printf("Topic:     %s\n", val.Topic)
	fmt.Printf("Payload:   %s\n", payloadStr)
	fmt.Printf("Format:    %s\n", formatStr)
	fmt.Printf("Timestamp: %s\n", timeStr)
	fmt.Printf("QoS:       %d\n", val.QoS)
	return nil
}

func runSetValue(ctx context.Context, client *Client, args []string) error {
	if len(args) < 2 || hasHelpFlag(args) {
		fmt.Println("Usage: mmq publish <topic> <payload> [options]")
		fmt.Println()
		fmt.Println("Publish a message payload to a topic.")
		fmt.Println()
		fmt.Println("Arguments:")
		fmt.Println("  <topic>             Topic name (required)")
		fmt.Println("  <payload>           Message payload string or JSON (required)")
		fmt.Println()
		fmt.Println("Options:")
		fmt.Println("  --retain, -r        Publish as retained message (default: false)")
		fmt.Println("  --qos 0|1|2         MQTT QoS level (default: 0)")
		fmt.Println("  -h, --help          Show this help text")
		return nil
	}
	topic := args[0]
	payload := args[1]
	retain := false
	qos := 0

	for i := 2; i < len(args); i++ {
		switch args[i] {
		case "--retain":
			retain = true
		case "--qos":
			if i+1 < len(args) {
				qos, _ = strconv.Atoi(args[i+1])
				i++
			}
		}
	}

	query := `
		mutation Publish($topic: String!, $payload: String!, $qos: Int, $retained: Boolean) {
			publish(input: { topic: $topic, payload: $payload, qos: $qos, retained: $retained }) {
				success
				error
				topic
				timestamp
			}
		}
	`
	var res struct {
		Data struct {
			Publish struct {
				Success   bool   `json:"success"`
				Error     string `json:"error"`
				Topic     string `json:"topic"`
				Timestamp int64  `json:"timestamp"`
			} `json:"publish"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}

	if err := client.DoQuery(ctx, query, map[string]any{
		"topic":    topic,
		"payload":  payload,
		"qos":      qos,
		"retained": retain,
	}, &res); err != nil {
		return err
	}
	if len(res.Errors) > 0 {
		return fmt.Errorf("GraphQL error: %s", res.Errors[0].Message)
	}

	if client.cfg.JSONMode {
		return printJSON(res.Data.Publish)
	}

	if res.Data.Publish.Success {
		fmt.Printf("Published to topic '%s' successfully\n", topic)
	} else {
		fmt.Printf("Failed to publish: %s\n", res.Data.Publish.Error)
	}
	return nil
}

func runListTopics(ctx context.Context, client *Client, args []string) error {
	if hasHelpFlag(args) {
		fmt.Println("Usage: mmq searchTopics [pattern] [options]")
		fmt.Println()
		fmt.Println("Search active topics across database archives and live retained store.")
		fmt.Println()
		fmt.Println("Arguments:")
		fmt.Println("  [pattern]           Glob (*), SQL LIKE (%), or MQTT (#) wildcard (default: #)")
		fmt.Println()
		fmt.Println("Options:")
		fmt.Println("  --limit N, -l N     Maximum topics to return (default: 100)")
		fmt.Println("  --archive-group, -g Archive group name (default: Default)")
		fmt.Println("  -h, --help          Show this help text")
		return nil
	}

	pattern := "#"
	limit := 100
	archiveGroup := "Default"

	if len(args) > 0 && !strings.HasPrefix(args[0], "--") {
		pattern = args[0]
	}

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--limit":
			if i+1 < len(args) {
				limit, _ = strconv.Atoi(args[i+1])
				i++
			}
		case "--archive-group":
			if i+1 < len(args) {
				archiveGroup = args[i+1]
				i++
			}
		}
	}

	// Normalize wildcard syntax for SQL LIKE queries (% and _)
	sqlPattern := pattern
	if strings.Contains(sqlPattern, "*") {
		sqlPattern = strings.ReplaceAll(sqlPattern, "*", "%")
	} else if sqlPattern != "#" && sqlPattern != "%" && !strings.Contains(sqlPattern, "%") {
		sqlPattern = "%" + sqlPattern + "%"
	}

	// Extract search substring for case-insensitive client-side filtering
	searchTerm := strings.Trim(pattern, "*%#")

	topicSet := make(map[string]bool)
	var matchedTopics []string

	// 1. Query searchTopics (Database Archive Store)
	searchTopicsQuery := `
		query SearchTopics($pattern: String!, $limit: Int, $archiveGroup: String) {
			searchTopics(pattern: $pattern, limit: $limit, archiveGroup: $archiveGroup)
		}
	`
	var stRes struct {
		Data struct {
			SearchTopics []string `json:"searchTopics"`
		} `json:"data"`
	}
	stErr := client.DoQuery(ctx, searchTopicsQuery, map[string]any{
		"pattern":      sqlPattern,
		"limit":        limit,
		"archiveGroup": archiveGroup,
	}, &stRes)
	if stErr == nil {
		for _, t := range stRes.Data.SearchTopics {
			if t != "" && !topicSet[t] {
				topicSet[t] = true
				matchedTopics = append(matchedTopics, t)
			}
		}
	}

	// 2. Query retainedMessages (Live Retained RAM Store)
	retainedFilter := "#"
	if strings.Contains(pattern, "/") && !strings.Contains(pattern, "*") && !strings.Contains(pattern, "%") {
		retainedFilter = pattern
	}
	retainedQuery := `
		query RetainedTopics($filter: String, $limit: Int) {
			retainedMessages(topicFilter: $filter, limit: $limit) {
				topic
			}
		}
	`
	var rmRes struct {
		Data struct {
			RetainedMessages []struct {
				Topic string `json:"topic"`
			} `json:"retainedMessages"`
		} `json:"data"`
	}
	rmErr := client.DoQuery(ctx, retainedQuery, map[string]any{
		"filter": retainedFilter,
		"limit":  limit,
	}, &rmRes)
	if rmErr == nil {
		for _, rm := range rmRes.Data.RetainedMessages {
			if rm.Topic != "" && !topicSet[rm.Topic] {
				if isTopicMatch(rm.Topic, pattern, searchTerm) {
					topicSet[rm.Topic] = true
					matchedTopics = append(matchedTopics, rm.Topic)
				}
			}
		}
	}

	if stErr != nil && rmErr != nil {
		return stErr
	}

	if limit > 0 && len(matchedTopics) > limit {
		matchedTopics = matchedTopics[:limit]
	}

	if client.cfg.JSONMode {
		return printJSON(matchedTopics)
	}

	fmt.Printf("Found %d topics matching '%s':\n", len(matchedTopics), pattern)
	for _, t := range matchedTopics {
		fmt.Println(" -", t)
	}
	return nil
}

func isTopicMatch(topic, pattern, searchTerm string) bool {
	if pattern == "#" || pattern == "%" || pattern == "*" || pattern == "" {
		return true
	}
	if searchTerm != "" {
		return strings.Contains(strings.ToLower(topic), strings.ToLower(searchTerm))
	}
	return true
}

func runListArchives(ctx context.Context, client *Client, args []string) error {
	if hasHelpFlag(args) {
		fmt.Println("Usage: mmq archiveGroups [options]")
		fmt.Println()
		fmt.Println("List all deployed archive groups and their storage configurations.")
		fmt.Println()
		fmt.Println("Options:")
		fmt.Println("  -h, --help          Show this help text")
		return nil
	}
	query := `
		query ListArchives {
			archiveGroups {
				name
				enabled
				deployed
				topicFilter
				retainedOnly
				lastValType
				archiveType
				databaseConnectionName
			}
		}
	`
	var res struct {
		Data struct {
			ArchiveGroups []struct {
				Name                   string   `json:"name"`
				Enabled                bool     `json:"enabled"`
				Deployed               bool     `json:"deployed"`
				TopicFilter            []string `json:"topicFilter"`
				RetainedOnly           bool     `json:"retainedOnly"`
				LastValType            string   `json:"lastValType"`
				ArchiveType            string   `json:"archiveType"`
				DatabaseConnectionName string   `json:"databaseConnectionName"`
			} `json:"archiveGroups"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}

	if err := client.DoQuery(ctx, query, nil, &res); err != nil {
		return err
	}
	if len(res.Errors) > 0 {
		return fmt.Errorf("GraphQL error: %s", res.Errors[0].Message)
	}

	if client.cfg.JSONMode {
		return printJSON(res.Data.ArchiveGroups)
	}

	fmt.Printf("%-20s %-10s %-12s %-12s %-20s\n", "NAME", "ENABLED", "LASTVAL_TYPE", "ARCHIVE_TYPE", "FILTERS")
	fmt.Println(strings.Repeat("-", 78))
	for _, g := range res.Data.ArchiveGroups {
		fmt.Printf("%-20s %-10t %-12s %-12s %-20s\n", g.Name, g.Enabled, g.LastValType, g.ArchiveType, strings.Join(g.TopicFilter, ", "))
	}
	return nil
}

func runArchiveStats(ctx context.Context, client *Client, args []string) error {
	if len(args) < 1 || hasHelpFlag(args) {
		fmt.Println("Usage: mmq archiveStats <group> [options]")
		fmt.Println()
		fmt.Println("Display min timestamps and daily message counts for an archive group.")
		fmt.Println()
		fmt.Println("Arguments:")
		fmt.Println("  <group>                  Archive group name (required)")
		fmt.Println()
		fmt.Println("Options:")
		fmt.Println("  --start ISO_TIME         Start time in ISO 8601 UTC")
		fmt.Println("  --end ISO_TIME           End time in ISO 8601 UTC")
		fmt.Println("  --last-seconds N, -s N   Query stats for the last N seconds")
		fmt.Println("  -h, --help               Show this help text")
		return nil
	}
	group := args[0]
	var startTime, endTime string

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--start":
			if i+1 < len(args) {
				startTime = args[i+1]
				i++
			}
		case "--end":
			if i+1 < len(args) {
				endTime = args[i+1]
				i++
			}
		case "--last-seconds":
			if i+1 < len(args) {
				sec, _ := strconv.Atoi(args[i+1])
				now := time.Now().UTC()
				start := now.Add(-time.Duration(sec) * time.Second)
				startTime = start.Format(time.RFC3339)
				endTime = now.Format(time.RFC3339)
				i++
			}
		}
	}

	query := `
		query ArchiveStats($group: String!, $startTime: String, $endTime: String) {
			archiveStats(archiveGroup: $group, startTime: $startTime, endTime: $endTime) {
				minTimestamp
				dailyCounts {
					date
					count
				}
			}
		}
	`
	var res struct {
		Data struct {
			ArchiveStats *struct {
				MinTimestamp string `json:"minTimestamp"`
				DailyCounts  []struct {
					Date  string `json:"date"`
					Count int64  `json:"count"`
				} `json:"dailyCounts"`
			} `json:"archiveStats"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}

	vars := map[string]any{
		"group": group,
	}
	if startTime != "" {
		vars["startTime"] = startTime
	}
	if endTime != "" {
		vars["endTime"] = endTime
	}

	if err := client.DoQuery(ctx, query, vars, &res); err != nil {
		return err
	}
	if len(res.Errors) > 0 {
		return fmt.Errorf("GraphQL error: %s", res.Errors[0].Message)
	}

	if client.cfg.JSONMode {
		return printJSON(res.Data.ArchiveStats)
	}

	stats := res.Data.ArchiveStats
	if stats == nil {
		fmt.Printf("No stats available for group '%s'\n", group)
		return nil
	}

	fmt.Printf("Min Timestamp: %s\n", stats.MinTimestamp)
	fmt.Println("Daily Message Counts:")
	for _, dc := range stats.DailyCounts {
		fmt.Printf("  %-12s %d\n", dc.Date, dc.Count)
	}
	return nil
}

func runQueryHistory(ctx context.Context, client *Client, args []string) error {
	if len(args) < 1 || hasHelpFlag(args) {
		fmt.Println("Usage: mmq archivedMessages <topic> [archiveGroup] [options]")
		fmt.Println()
		fmt.Println("Query historical messages for a topic from an archive group.")
		fmt.Println("By default, queries the last 60 seconds (1 minute) from the 'Default' archive group.")
		fmt.Println()
		fmt.Println("Arguments:")
		fmt.Println("  <topic>                  Topic name or MQTT filter (required)")
		fmt.Println("  [archiveGroup]           Archive group name (default: Default)")
		fmt.Println()
		fmt.Println("Options:")
		fmt.Println("  --start ISO_TIME         Start time in ISO 8601 UTC (e.g. 2026-08-14T00:00:00Z)")
		fmt.Println("  --end ISO_TIME           End time in ISO 8601 UTC")
		fmt.Println("  --last-seconds N, -s N   Query messages from the last N seconds (default: 60)")
		fmt.Println("  --limit N, -l N          Maximum messages to retrieve (default: 100)")
		fmt.Println("  --archive-group, -g      Archive group name (alternative to positional argument)")
		fmt.Println("  -h, --help               Show this help text")
		return nil
	}

	var topic string
	archiveGroup := "Default"
	limit := 100
	var startTime, endTime string

	var nonFlagArgs []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "-") {
			switch arg {
			case "--start":
				if i+1 < len(args) {
					startTime = args[i+1]
					i++
				}
			case "--end":
				if i+1 < len(args) {
					endTime = args[i+1]
					i++
				}
			case "--last-seconds", "-s":
				if i+1 < len(args) {
					sec, _ := strconv.Atoi(args[i+1])
					now := time.Now().UTC()
					start := now.Add(-time.Duration(sec) * time.Second)
					startTime = start.Format(time.RFC3339)
					endTime = now.Format(time.RFC3339)
					i++
				}
			case "--limit", "-l":
				if i+1 < len(args) {
					limit, _ = strconv.Atoi(args[i+1])
					i++
				}
			case "--archive-group", "--group", "-g":
				if i+1 < len(args) {
					archiveGroup = args[i+1]
					i++
				}
			}
		} else {
			nonFlagArgs = append(nonFlagArgs, arg)
		}
	}

	if len(nonFlagArgs) > 0 {
		topic = nonFlagArgs[0]
	}
	if len(nonFlagArgs) > 1 {
		archiveGroup = nonFlagArgs[1]
	}

	if topic == "" {
		fmt.Println("Usage: mmq archivedMessages <topic> [archiveGroup] [options]")
		return nil
	}

	// Default to last 60 seconds if no timerange or last-seconds is specified
	if startTime == "" && endTime == "" {
		now := time.Now().UTC()
		start := now.Add(-60 * time.Second)
		startTime = start.Format(time.RFC3339)
		endTime = now.Format(time.RFC3339)
	}

	query := `
		query QueryHistory($topic: String!, $startTime: String, $endTime: String, $limit: Int, $archiveGroup: String!) {
			archivedMessages(topicFilter: $topic, startTime: $startTime, endTime: $endTime, limit: $limit, archiveGroup: $archiveGroup) {
				topic
				payload
				format
				timestamp
				qos
				clientId
			}
		}
	`
	var res struct {
		Data struct {
			ArchivedMessages []struct {
				Topic     string `json:"topic"`
				Payload   string `json:"payload"`
				Format    string `json:"format"`
				Timestamp int64  `json:"timestamp"`
				QoS       int    `json:"qos"`
				ClientID  string `json:"clientId"`
			} `json:"archivedMessages"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}

	vars := map[string]any{
		"topic":        topic,
		"limit":        limit,
		"archiveGroup": archiveGroup,
	}
	if startTime != "" {
		vars["startTime"] = startTime
	}
	if endTime != "" {
		vars["endTime"] = endTime
	}

	if err := client.DoQuery(ctx, query, vars, &res); err != nil {
		return err
	}
	if len(res.Errors) > 0 {
		return fmt.Errorf("GraphQL error: %s", res.Errors[0].Message)
	}

	if client.cfg.JSONMode {
		return printJSON(res.Data.ArchivedMessages)
	}

	fmt.Printf("Retrieved %d historical messages:\n", len(res.Data.ArchivedMessages))
	for _, m := range res.Data.ArchivedMessages {
		isoTime := formatTimestampISO(m.Timestamp)
		timeStr := strconv.FormatInt(m.Timestamp, 10)
		if isoTime != "" {
			timeStr = isoTime
		}
		fmt.Printf(" [%s] %s: %s\n", timeStr, m.Topic, formatPayload(m.Payload))
	}
	return nil
}

func runQueryAggregated(ctx context.Context, client *Client, args []string) error {
	if len(args) < 1 || hasHelpFlag(args) {
		fmt.Println("Usage: mmq aggregatedMessages <topic1> [topic2...] [options]")
		fmt.Println()
		fmt.Println("Query aggregated time-series metrics (AVG, MIN, MAX, COUNT) across topics.")
		fmt.Println()
		fmt.Println("Arguments:")
		fmt.Println("  <topics...>              One or more topic names (required)")
		fmt.Println()
		fmt.Println("Options:")
		fmt.Println("  --interval INTERVAL      Time bucket: ONE_MINUTE, FIVE_MINUTES, FIFTEEN_MINUTES, ONE_HOUR, ONE_DAY (default: ONE_MINUTE)")
		fmt.Println("  --functions FN1,FN2      Aggregate functions: AVG, MIN, MAX, COUNT (default: AVG)")
		fmt.Println("  --fields FIELD1,FIELD2   JSON fields to aggregate")
		fmt.Println("  --start ISO_TIME         Start time in ISO 8601 UTC")
		fmt.Println("  --end ISO_TIME           End time in ISO 8601 UTC")
		fmt.Println("  --last-seconds N, -s N   Time window in seconds")
		fmt.Println("  --archive-group, -g      Archive group name (default: Default)")
		fmt.Println("  -h, --help               Show this help text")
		return nil
	}
	var topics []string
	interval := "ONE_MINUTE"
	functions := []string{"AVG"}
	var fields []string
	archiveGroup := "Default"
	var startTime, endTime string

	for i := 0; i < len(args); i++ {
		if strings.HasPrefix(args[i], "--") {
			switch args[i] {
			case "--interval":
				if i+1 < len(args) {
					interval = args[i+1]
					i++
				}
			case "--functions":
				if i+1 < len(args) {
					functions = strings.Split(args[i+1], ",")
					i++
				}
			case "--fields":
				if i+1 < len(args) {
					fields = strings.Split(args[i+1], ",")
					i++
				}
			case "--last-seconds":
				if i+1 < len(args) {
					sec, _ := strconv.Atoi(args[i+1])
					now := time.Now().UTC()
					start := now.Add(-time.Duration(sec) * time.Second)
					startTime = start.Format(time.RFC3339)
					endTime = now.Format(time.RFC3339)
					i++
				}
			case "--start":
				if i+1 < len(args) {
					startTime = args[i+1]
					i++
				}
			case "--end":
				if i+1 < len(args) {
					endTime = args[i+1]
					i++
				}
			case "--archive-group":
				if i+1 < len(args) {
					archiveGroup = args[i+1]
					i++
				}
			}
		} else {
			topics = append(topics, args[i])
		}
	}

	if startTime == "" || endTime == "" {
		now := time.Now().UTC()
		start := now.Add(-1 * time.Hour)
		startTime = start.Format(time.RFC3339)
		endTime = now.Format(time.RFC3339)
	}

	query := `
		query QueryAggregated($topics: [String!]!, $interval: AggregationInterval!, $startTime: String!, $endTime: String!, $functions: [AggregationFunction!], $fields: [String!], $archiveGroup: String!) {
			aggregatedMessages(topics: $topics, interval: $interval, startTime: $startTime, endTime: $endTime, functions: $functions, fields: $fields, archiveGroup: $archiveGroup) {
				columns
				rows
				interval
				startTime
				endTime
				topicCount
				rowCount
			}
		}
	`
	var res struct {
		Data struct {
			AggregatedMessages *struct {
				Columns    []string `json:"columns"`
				Rows       [][]any  `json:"rows"`
				Interval   string   `json:"interval"`
				StartTime  string   `json:"startTime"`
				EndTime    string   `json:"endTime"`
				TopicCount int      `json:"topicCount"`
				RowCount   int      `json:"rowCount"`
			} `json:"aggregatedMessages"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}

	vars := map[string]any{
		"topics":       topics,
		"interval":     interval,
		"startTime":    startTime,
		"endTime":      endTime,
		"functions":    functions,
		"archiveGroup": archiveGroup,
	}
	if len(fields) > 0 {
		vars["fields"] = fields
	}

	if err := client.DoQuery(ctx, query, vars, &res); err != nil {
		return err
	}
	if len(res.Errors) > 0 {
		return fmt.Errorf("GraphQL error: %s", res.Errors[0].Message)
	}

	if client.cfg.JSONMode {
		return printJSON(res.Data.AggregatedMessages)
	}

	agg := res.Data.AggregatedMessages
	if agg == nil {
		fmt.Println("No aggregated data returned.")
		return nil
	}

	fmt.Println("Columns:", strings.Join(agg.Columns, " | "))
	fmt.Println(strings.Repeat("-", 60))
	for _, row := range agg.Rows {
		var strRow []string
		for _, val := range row {
			strRow = append(strRow, fmt.Sprintf("%v", val))
		}
		fmt.Println(strings.Join(strRow, " | "))
	}
	return nil
}

func runDeviceList(ctx context.Context, client *Client, args []string) error {
	if hasHelpFlag(args) {
		fmt.Println("Usage: mmq device list [type] [options]")
		fmt.Println()
		fmt.Println("List all configured devices and edge nodes.")
		fmt.Println()
		fmt.Println("Arguments:")
		fmt.Println("  [type]                   Filter devices by type (e.g. OPCUA_CLIENT, MQTT_CLIENT, KAFKA_CLIENT)")
		fmt.Println()
		fmt.Println("Options:")
		fmt.Println("  --type <type>, -t <type> Filter devices by type")
		fmt.Println("  -h, --help               Show this help text")
		return nil
	}

	filterType := ""
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if (arg == "--type" || arg == "-t") && i+1 < len(args) {
			filterType = args[i+1]
			i++
		} else if !strings.HasPrefix(arg, "-") && filterType == "" {
			filterType = arg
		}
	}

	query := `
		query ListDevices {
			getDevices {
				name
				namespace
				nodeId
				type
				enabled
				createdAt
				updatedAt
			}
		}
	`
	var res struct {
		Data struct {
			GetDevices []struct {
				Name      string `json:"name"`
				Namespace string `json:"namespace"`
				NodeID    string `json:"nodeId"`
				Type      string `json:"type"`
				Enabled   bool   `json:"enabled"`
				CreatedAt string `json:"createdAt"`
				UpdatedAt string `json:"updatedAt"`
			} `json:"getDevices"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}

	if err := client.DoQuery(ctx, query, nil, &res); err != nil {
		return err
	}
	if len(res.Errors) > 0 {
		return fmt.Errorf("GraphQL error: %s", res.Errors[0].Message)
	}

	devices := res.Data.GetDevices
	if filterType != "" {
		filtered := make([]struct {
			Name      string `json:"name"`
			Namespace string `json:"namespace"`
			NodeID    string `json:"nodeId"`
			Type      string `json:"type"`
			Enabled   bool   `json:"enabled"`
			CreatedAt string `json:"createdAt"`
			UpdatedAt string `json:"updatedAt"`
		}, 0)
		ft := strings.ToLower(filterType)
		for _, d := range devices {
			if strings.Contains(strings.ToLower(d.Type), ft) {
				filtered = append(filtered, d)
			}
		}
		devices = filtered
	}

	if client.cfg.JSONMode {
		return printJSON(devices)
	}

	if len(devices) == 0 {
		if filterType != "" {
			fmt.Printf("No devices found matching type '%s'\n", filterType)
		} else {
			fmt.Println("No devices found")
		}
		return nil
	}

	fmt.Printf("%-20s %-15s %-15s %-15s %-10s %-20s\n", "NAME", "NAMESPACE", "NODE_ID", "TYPE", "ENABLED", "UPDATED_AT")
	fmt.Println(strings.Repeat("-", 98))
	for _, d := range devices {
		fmt.Printf("%-20s %-15s %-15s %-15s %-10t %-20s\n", d.Name, d.Namespace, d.NodeID, d.Type, d.Enabled, d.UpdatedAt)
	}
	return nil
}

func runDeviceDownload(ctx context.Context, client *Client, args []string) error {
	if hasHelpFlag(args) {
		fmt.Println("Usage: mmq device download [device-name] [output-file.json]")
		fmt.Println()
		fmt.Println("Export device JSON configurations to standard output or a file.")
		return nil
	}
	name := ""
	outFile := ""
	if len(args) > 0 {
		name = args[0]
	}
	if len(args) > 1 {
		outFile = args[1]
	}

	query := `
		query GetDevice($names: [String!]) {
			getDevices(names: $names) {
				name
				namespace
				nodeId
				type
				enabled
				config
				createdAt
				updatedAt
			}
		}
	`
	var res struct {
		Data struct {
			GetDevices []struct {
				Name      string `json:"name"`
				Namespace string `json:"namespace"`
				NodeID    string `json:"nodeId"`
				Type      string `json:"type"`
				Enabled   bool   `json:"enabled"`
				Config    any    `json:"config"`
				CreatedAt string `json:"createdAt"`
				UpdatedAt string `json:"updatedAt"`
			} `json:"getDevices"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}

	vars := map[string]any{}
	if name != "" {
		vars["names"] = []string{name}
	}

	if err := client.DoQuery(ctx, query, vars, &res); err != nil {
		return err
	}
	if len(res.Errors) > 0 {
		return fmt.Errorf("GraphQL error: %s", res.Errors[0].Message)
	}

	jsonBytes, err := json.MarshalIndent(res.Data.GetDevices, "", "  ")
	if err != nil {
		return err
	}

	if outFile != "" {
		if err := os.WriteFile(outFile, jsonBytes, 0644); err != nil {
			return fmt.Errorf("error writing output file: %w", err)
		}
		fmt.Printf("Device configuration saved to '%s'\n", outFile)
		return nil
	}

	fmt.Println(string(jsonBytes))
	return nil
}

func runDeviceUpload(ctx context.Context, client *Client, args []string) error {
	if len(args) < 1 || hasHelpFlag(args) {
		fmt.Println("Usage: mmq device upload <file.json>")
		fmt.Println()
		fmt.Println("Import or bulk update device configurations from a JSON file.")
		return nil
	}
	inFile := args[0]

	fileBytes, err := os.ReadFile(inFile)
	if err != nil {
		return fmt.Errorf("error reading device config file: %w", err)
	}

	var configs []any
	if err := json.Unmarshal(fileBytes, &configs); err != nil {
		var singleConfig any
		if err2 := json.Unmarshal(fileBytes, &singleConfig); err2 == nil {
			configs = []any{singleConfig}
		} else {
			return fmt.Errorf("invalid JSON in device config file: %w", err)
		}
	}

	query := `
		mutation ImportDevices($configs: [DeviceInput!]!) {
			importDevices(configs: $configs) {
				success
				imported
				failed
				total
				errors
			}
		}
	`
	var res struct {
		Data struct {
			ImportDevices struct {
				Success  bool     `json:"success"`
				Imported int      `json:"imported"`
				Failed   int      `json:"failed"`
				Total    int      `json:"total"`
				Errors   []string `json:"errors"`
			} `json:"importDevices"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}

	if err := client.DoQuery(ctx, query, map[string]any{
		"configs": configs,
	}, &res); err != nil {
		return err
	}
	if len(res.Errors) > 0 {
		return fmt.Errorf("GraphQL error: %s", res.Errors[0].Message)
	}

	if client.cfg.JSONMode {
		return printJSON(res.Data.ImportDevices)
	}

	imp := res.Data.ImportDevices
	if imp.Success {
		fmt.Printf("Device configurations imported successfully: %d/%d configs imported\n", imp.Imported, imp.Total)
	} else {
		fmt.Printf("Device configuration import completed with errors: %d failed\n", imp.Failed)
		for _, e := range imp.Errors {
			fmt.Println(" - Error:", e)
		}
	}
	return nil
}

func runDeviceEnable(ctx context.Context, client *Client, args []string) error {
	return setDeviceEnabled(ctx, client, args, true)
}

func runDeviceDisable(ctx context.Context, client *Client, args []string) error {
	return setDeviceEnabled(ctx, client, args, false)
}

func setDeviceEnabled(ctx context.Context, client *Client, args []string, enabled bool) error {
	actionStr := "enable"
	if !enabled {
		actionStr = "disable"
	}
	if len(args) < 1 || hasHelpFlag(args) {
		fmt.Printf("Usage: mmq device %s <name>\n\n", actionStr)
		fmt.Printf("%s a configured device or edge MQTT client dynamically.\n", strings.Title(actionStr))
		return nil
	}
	name := args[0]

	getGql := `
		query GetDevice($names: [String!]) {
			getDevices(names: $names) {
				name
				namespace
				nodeId
				type
				enabled
				config
			}
		}
	`
	var getRes struct {
		Data struct {
			GetDevices []struct {
				Name      string `json:"name"`
				Namespace string `json:"namespace"`
				NodeID    string `json:"nodeId"`
				Type      string `json:"type"`
				Enabled   bool   `json:"enabled"`
				Config    any    `json:"config"`
			} `json:"getDevices"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}

	if err := client.DoQuery(ctx, getGql, map[string]any{"names": []string{name}}, &getRes); err != nil {
		return err
	}
	if len(getRes.Errors) > 0 {
		return fmt.Errorf("GraphQL error: %s", getRes.Errors[0].Message)
	}
	if len(getRes.Data.GetDevices) == 0 {
		return fmt.Errorf("device '%s' not found", name)
	}

	dev := getRes.Data.GetDevices[0]
	updatedInput := map[string]any{
		"name":      dev.Name,
		"namespace": dev.Namespace,
		"nodeId":    dev.NodeID,
		"type":      dev.Type,
		"enabled":   enabled,
		"config":    dev.Config,
	}

	importGql := `
		mutation ImportDevices($configs: [DeviceInput!]!) {
			importDevices(configs: $configs) {
				success
				imported
				failed
				total
				errors
			}
		}
	`
	var importRes struct {
		Data struct {
			ImportDevices struct {
				Success  bool     `json:"success"`
				Imported int      `json:"imported"`
				Failed   int      `json:"failed"`
				Total    int      `json:"total"`
				Errors   []string `json:"errors"`
			} `json:"importDevices"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}

	if err := client.DoQuery(ctx, importGql, map[string]any{"configs": []any{updatedInput}}, &importRes); err != nil {
		return err
	}

	if strings.EqualFold(dev.Type, "MQTT-Client") || strings.EqualFold(dev.Type, "MQTT_CLIENT") {
		toggleGql := `
			mutation ToggleMqttClient($name: String!, $enabled: Boolean!) {
				mqttClient {
					toggle(name: $name, enabled: $enabled) {
						success
					}
				}
			}
		`
		var toggleRes struct {
			Data struct {
				MqttClient struct {
					Toggle struct {
						Success bool `json:"success"`
					} `json:"toggle"`
				} `json:"mqttClient"`
			} `json:"data"`
		}
		if err := client.DoQuery(ctx, toggleGql, map[string]any{
			"name":    name,
			"enabled": enabled,
		}, &toggleRes); err != nil {
			return err
		}
	}

	statusStr := "enabled"
	if !enabled {
		statusStr = "disabled"
	}

	if client.cfg.JSONMode {
		return printJSON(map[string]any{
			"device":  name,
			"enabled": enabled,
			"success": true,
		})
	}

	fmt.Printf("Device '%s' %s successfully\n", name, statusStr)
	return nil
}

func runListFeatures(ctx context.Context, client *Client, args []string) error {
	if hasHelpFlag(args) {
		fmt.Println("Usage: mmq features [options]")
		fmt.Println()
		fmt.Println("Query the connected broker instance to list active feature flags.")
		fmt.Println()
		fmt.Println("Options:")
		fmt.Println("  -h, --help          Show this help text")
		return nil
	}
	query := `
		query {
			broker {
				enabledFeatures
			}
		}
	`
	var res struct {
		Data struct {
			Broker struct {
				EnabledFeatures []string `json:"enabledFeatures"`
			} `json:"broker"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}

	if err := client.DoQuery(ctx, query, nil, &res); err != nil {
		return err
	}
	if len(res.Errors) > 0 {
		return fmt.Errorf("GraphQL error: %s", res.Errors[0].Message)
	}

	if client.cfg.JSONMode {
		return printJSON(res.Data.Broker.EnabledFeatures)
	}

	if len(res.Data.Broker.EnabledFeatures) == 0 {
		fmt.Println("No features enabled")
		return nil
	}

	fmt.Println("ENABLED FEATURES")
	fmt.Println(strings.Repeat("-", 20))
	for _, f := range res.Data.Broker.EnabledFeatures {
		fmt.Println(f)
	}
	return nil
}

func printJSON(v any) error {
	bytes, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(bytes))
	return nil
}

func runGetValues(ctx context.Context, client *Client, args []string) error {
	if len(args) < 1 || hasHelpFlag(args) {
		fmt.Println("Usage: mmq currentValues <topic-filter> [options]")
		fmt.Println()
		fmt.Println("Fetch current values for all topics matching an MQTT topic filter.")
		fmt.Println()
		fmt.Println("Arguments:")
		fmt.Println("  <topic-filter>           MQTT topic filter e.g. sensors/# (required)")
		fmt.Println()
		fmt.Println("Options:")
		fmt.Println("  --limit N, -l N          Maximum topics to return (default: 100)")
		fmt.Println("  --archive-group, -g      Archive group name (default: Default)")
		fmt.Println("  -h, --help               Show this help text")
		return nil
	}
	filter := args[0]
	limit := 100
	archiveGroup := "Default"

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--limit":
			if i+1 < len(args) {
				limit, _ = strconv.Atoi(args[i+1])
				i++
			}
		case "--archive-group":
			if i+1 < len(args) {
				archiveGroup = args[i+1]
				i++
			}
		}
	}

	query := `
		query GetValues($filter: String!, $limit: Int, $archiveGroup: String) {
			currentValues(topicFilter: $filter, limit: $limit, archiveGroup: $archiveGroup) {
				topic
				payload
				format
				timestamp
				qos
			}
		}
	`
	var res struct {
		Data struct {
			CurrentValues []struct {
				Topic     string `json:"topic"`
				Payload   string `json:"payload"`
				Format    string `json:"format"`
				Timestamp int64  `json:"timestamp"`
				QoS       int    `json:"qos"`
			} `json:"currentValues"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}

	if err := client.DoQuery(ctx, query, map[string]any{
		"filter":       filter,
		"limit":        limit,
		"archiveGroup": archiveGroup,
	}, &res); err != nil {
		return err
	}
	if len(res.Errors) > 0 {
		return fmt.Errorf("GraphQL error: %s", res.Errors[0].Message)
	}

	if client.cfg.JSONMode {
		return printJSON(res.Data.CurrentValues)
	}

	fmt.Printf("Found %d values matching '%s':\n", len(res.Data.CurrentValues), filter)
	for _, val := range res.Data.CurrentValues {
		isoTime := formatTimestampISO(val.Timestamp)
		timeStr := strconv.FormatInt(val.Timestamp, 10)
		if isoTime != "" {
			timeStr = isoTime
		}
		fmt.Printf(" - Topic:     %s\n", val.Topic)
		fmt.Printf("   Payload:   %s\n", formatPayload(val.Payload))
		fmt.Printf("   Timestamp: %s\n", timeStr)
	}
	return nil
}

func runListRetained(ctx context.Context, client *Client, args []string) error {
	if hasHelpFlag(args) {
		fmt.Println("Usage: mmq retainedMessages [topic-filter] [options]")
		fmt.Println()
		fmt.Println("List all retained messages matching an MQTT topic filter.")
		fmt.Println()
		fmt.Println("Arguments:")
		fmt.Println("  [topic-filter]           MQTT topic filter (default: #)")
		fmt.Println()
		fmt.Println("Options:")
		fmt.Println("  --limit N, -l N          Maximum messages to return (default: 100)")
		fmt.Println("  -h, --help               Show this help text")
		return nil
	}
	filter := "#"
	limit := 100

	if len(args) > 0 && !strings.HasPrefix(args[0], "--") {
		filter = args[0]
	}

	for i := 0; i < len(args); i++ {
		if args[i] == "--limit" && i+1 < len(args) {
			limit, _ = strconv.Atoi(args[i+1])
			i++
		}
	}

	query := `
		query ListRetained($filter: String, $limit: Int) {
			retainedMessages(topicFilter: $filter, limit: $limit) {
				topic
				payload
				format
				timestamp
				qos
			}
		}
	`
	var res struct {
		Data struct {
			RetainedMessages []struct {
				Topic     string `json:"topic"`
				Payload   string `json:"payload"`
				Format    string `json:"format"`
				Timestamp int64  `json:"timestamp"`
				QoS       int    `json:"qos"`
			} `json:"retainedMessages"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}

	if err := client.DoQuery(ctx, query, map[string]any{
		"filter": filter,
		"limit":  limit,
	}, &res); err != nil {
		return err
	}
	if len(res.Errors) > 0 {
		return fmt.Errorf("GraphQL error: %s", res.Errors[0].Message)
	}

	if client.cfg.JSONMode {
		return printJSON(res.Data.RetainedMessages)
	}

	fmt.Printf("Found %d retained messages matching '%s':\n", len(res.Data.RetainedMessages), filter)
	for _, rm := range res.Data.RetainedMessages {
		isoTime := formatTimestampISO(rm.Timestamp)
		timeStr := strconv.FormatInt(rm.Timestamp, 10)
		if isoTime != "" {
			timeStr = isoTime
		}
		fmt.Printf(" - Topic:     %s\n", rm.Topic)
		fmt.Printf("   Payload:   %s\n", formatPayload(rm.Payload))
		fmt.Printf("   Timestamp: %s\n", timeStr)
	}
	return nil
}

func runBrowseTopics(ctx context.Context, client *Client, args []string) error {
	if hasHelpFlag(args) {
		fmt.Println("Usage: mmq browseTopics [path] [options]")
		fmt.Println()
		fmt.Println("Hierarchically browse topic levels.")
		fmt.Println()
		fmt.Println("Arguments:")
		fmt.Println("  [path]                   Topic path prefix (default: +)")
		fmt.Println()
		fmt.Println("Options:")
		fmt.Println("  --archive-group, -g      Archive group name (default: Default)")
		fmt.Println("  -h, --help               Show this help text")
		return nil
	}
	topic := "+"
	archiveGroup := "Default"

	if len(args) > 0 && !strings.HasPrefix(args[0], "--") {
		topic = args[0]
	}

	for i := 0; i < len(args); i++ {
		if args[i] == "--archive-group" && i+1 < len(args) {
			archiveGroup = args[i+1]
			i++
		}
	}

	// Normalize pattern if user passed glob wildcards (* -> +)
	if strings.Contains(topic, "*") {
		topic = strings.ReplaceAll(topic, "*", "+")
		topic = strings.Trim(topic, "+")
		if topic == "" {
			topic = "+"
		} else {
			topic = topic + "/+"
		}
	}

	query := `
		query BrowseTopics($topic: String!, $archiveGroup: String) {
			browseTopics(topic: $topic, archiveGroup: $archiveGroup) {
				name
				value {
					payload
					timestamp
				}
			}
		}
	`
	var res struct {
		Data struct {
			BrowseTopics []struct {
				Name  string `json:"name"`
				Value *struct {
					Payload   string `json:"payload"`
					Timestamp int64  `json:"timestamp"`
				} `json:"value"`
			} `json:"browseTopics"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}

	if err := client.DoQuery(ctx, query, map[string]any{
		"topic":        topic,
		"archiveGroup": archiveGroup,
	}, &res); err != nil {
		return err
	}
	if len(res.Errors) > 0 {
		return fmt.Errorf("GraphQL error: %s", res.Errors[0].Message)
	}

	if client.cfg.JSONMode {
		return printJSON(res.Data.BrowseTopics)
	}

	fmt.Printf("Topic hierarchy at '%s':\n", topic)
	for _, t := range res.Data.BrowseTopics {
		valStr := ""
		if t.Value != nil && t.Value.Payload != "" {
			valStr = fmt.Sprintf(" = %s", formatPayload(t.Value.Payload))
		}
		fmt.Printf(" - %s%s\n", t.Name, valStr)
	}
	return nil
}

func runSessionList(ctx context.Context, client *Client, args []string) error {
	if hasHelpFlag(args) {
		fmt.Println("Usage: mmq sessions")
		fmt.Println("       mmq session list")
		fmt.Println()
		fmt.Println("List all active MQTT client sessions.")
		return nil
	}
	query := `
		query ListSessions {
			sessions {
				clientId
				nodeId
				connected
				clientAddress
				queuedMessageCount
			}
		}
	`
	var res struct {
		Data struct {
			Sessions []struct {
				ClientId           string `json:"clientId"`
				NodeId             string `json:"nodeId"`
				Connected          bool   `json:"connected"`
				ClientAddress      string `json:"clientAddress"`
				QueuedMessageCount int64  `json:"queuedMessageCount"`
			} `json:"sessions"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}

	if err := client.DoQuery(ctx, query, nil, &res); err != nil {
		return err
	}
	if len(res.Errors) > 0 {
		return fmt.Errorf("GraphQL error: %s", res.Errors[0].Message)
	}

	if client.cfg.JSONMode {
		return printJSON(res.Data.Sessions)
	}

	fmt.Printf("%-28s %-12s %-20s %s\n", "CLIENT ID", "STATUS", "ADDRESS", "QUEUED")
	fmt.Println(strings.Repeat("-", 70))
	for _, s := range res.Data.Sessions {
		status := "DISCONNECTED"
		if s.Connected {
			status = "CONNECTED"
		}
		fmt.Printf("%-28s %-12s %-20s %d\n", s.ClientId, status, s.ClientAddress, s.QueuedMessageCount)
	}
	return nil
}

func runSessionInspect(ctx context.Context, client *Client, args []string) error {
	if len(args) < 1 || hasHelpFlag(args) {
		fmt.Println("Usage: mmq session <clientId>")
		fmt.Println("       mmq session inspect <clientId>")
		fmt.Println()
		fmt.Println("Inspect active MQTT client session and subscriptions.")
		fmt.Println()
		fmt.Println("Arguments:")
		fmt.Println("  <clientId>   Client identifier to inspect (required)")
		fmt.Println()
		fmt.Println("Options:")
		fmt.Println("  -h, --help   Show this help text")
		return nil
	}
	clientId := args[0]

	query := `
		query GetSession($clientId: String!) {
			session(clientId: $clientId) {
				clientId
				nodeId
				connected
				cleanSession
				clientAddress
				protocolVersion
				queuedMessageCount
				subscriptions {
					topicFilter
					qos
				}
			}
		}
	`
	var res struct {
		Data struct {
			Session *struct {
				ClientId           string `json:"clientId"`
				NodeId             string `json:"nodeId"`
				Connected          bool   `json:"connected"`
				CleanSession       bool   `json:"cleanSession"`
				ClientAddress      string `json:"clientAddress"`
				ProtocolVersion    int    `json:"protocolVersion"`
				QueuedMessageCount int64  `json:"queuedMessageCount"`
				Subscriptions      []struct {
					TopicFilter string `json:"topicFilter"`
					QoS         int    `json:"qos"`
				} `json:"subscriptions"`
			} `json:"session"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}

	if err := client.DoQuery(ctx, query, map[string]any{"clientId": clientId}, &res); err != nil {
		return err
	}
	if len(res.Errors) > 0 {
		return fmt.Errorf("GraphQL error: %s", res.Errors[0].Message)
	}

	if client.cfg.JSONMode {
		return printJSON(res.Data.Session)
	}
	if res.Data.Session == nil {
		fmt.Printf("No session found for Client ID '%s'\n", clientId)
		return nil
	}

	s := res.Data.Session
	fmt.Printf("Client ID:       %s\n", s.ClientId)
	fmt.Printf("Node ID:         %s\n", s.NodeId)
	fmt.Printf("Connected:       %t\n", s.Connected)
	fmt.Printf("Clean Session:   %t\n", s.CleanSession)
	fmt.Printf("Client Address:  %s\n", s.ClientAddress)
	fmt.Printf("Protocol Ver:    v%d\n", s.ProtocolVersion)
	fmt.Printf("Queued Messages: %d\n", s.QueuedMessageCount)
	fmt.Println("Subscriptions:")
	for _, sub := range s.Subscriptions {
		fmt.Printf(" - %s (QoS %d)\n", sub.TopicFilter, sub.QoS)
	}
	return nil
}

func runSessionRemove(ctx context.Context, client *Client, args []string) error {
	if len(args) < 1 || hasHelpFlag(args) {
		fmt.Println("Usage: mmq session remove <clientId1> [clientId2...]")
		fmt.Println()
		fmt.Println("Terminate and remove one or more client sessions.")
		return nil
	}

	query := `
		mutation RemoveSessions($clientIds: [String!]!) {
			sessions {
				removeSessions(clientIds: $clientIds) {
					details {
						clientId
						success
					}
				}
			}
		}
	`
	var res struct {
		Data struct {
			Sessions struct {
				RemoveSessions struct {
					Details []struct {
						ClientId string `json:"clientId"`
						Success  bool   `json:"success"`
					} `json:"details"`
				} `json:"removeSessions"`
			} `json:"sessions"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}

	if err := client.DoQuery(ctx, query, map[string]any{"clientIds": args}, &res); err != nil {
		return err
	}
	if len(res.Errors) > 0 {
		return fmt.Errorf("GraphQL error: %s", res.Errors[0].Message)
	}

	if client.cfg.JSONMode {
		return printJSON(res.Data.Sessions.RemoveSessions.Details)
	}

	for _, d := range res.Data.Sessions.RemoveSessions.Details {
		status := "failed"
		if d.Success {
			status = "removed"
		}
		fmt.Printf("Session '%s': %s\n", d.ClientId, status)
	}
	return nil
}

func runLogs(ctx context.Context, client *Client, args []string) error {
	if hasHelpFlag(args) {
		fmt.Println("Usage: mmq systemLogs [options]")
		fmt.Println()
		fmt.Println("View broker system logs.")
		fmt.Println()
		fmt.Println("Options:")
		fmt.Println("  --last-minutes N         Fetch logs from last N minutes (default: 60)")
		fmt.Println("  --limit N                Maximum log entries (default: 50)")
		fmt.Println("  -h, --help               Show this help text")
		return nil
	}
	limit := 50
	lastMinutes := 60

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--limit":
			if i+1 < len(args) {
				limit, _ = strconv.Atoi(args[i+1])
				i++
			}
		case "--last-minutes":
			if i+1 < len(args) {
				lastMinutes, _ = strconv.Atoi(args[i+1])
				i++
			}
		}
	}

	query := `
		query GetLogs($lastMinutes: Int, $limit: Int) {
			systemLogs(lastMinutes: $lastMinutes, limit: $limit) {
				timestamp
				level
				message
				logger
			}
		}
	`
	var res struct {
		Data struct {
			SystemLogs []struct {
				Timestamp string `json:"timestamp"`
				Level     string `json:"level"`
				Message   string `json:"message"`
				Logger    string `json:"logger"`
			} `json:"systemLogs"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}

	if err := client.DoQuery(ctx, query, map[string]any{
		"lastMinutes": lastMinutes,
		"limit":       limit,
	}, &res); err != nil {
		return err
	}
	if len(res.Errors) > 0 {
		return fmt.Errorf("GraphQL error: %s", res.Errors[0].Message)
	}

	if client.cfg.JSONMode {
		return printJSON(res.Data.SystemLogs)
	}

	fmt.Printf("System logs (last %d mins, max %d entries):\n", lastMinutes, limit)
	for _, l := range res.Data.SystemLogs {
		fmt.Printf("[%s] [%-5s] [%s] %s\n", l.Timestamp, l.Level, l.Logger, l.Message)
	}
	return nil
}

func runHmiList(ctx context.Context, client *Client, args []string) error {
	if hasHelpFlag(args) {
		fmt.Println("Usage: mmq hmis")
		fmt.Println()
		fmt.Println("List deployed HMI web dashboards.")
		return nil
	}
	query := `
		query ListHmis {
			hmis {
				name
				nodeId
				enabled
				isOnCurrentNode
				config {
					urlPath
					isMain
					title
					description
					entryPoint
				}
				fileCount
				sizeBytes
			}
		}
	`
	var res struct {
		Data struct {
			Hmis []struct {
				Name            string `json:"name"`
				NodeId          string `json:"nodeId"`
				Enabled         bool   `json:"enabled"`
				IsOnCurrentNode bool   `json:"isOnCurrentNode"`
				Config          struct {
					UrlPath     string `json:"urlPath"`
					IsMain      bool   `json:"isMain"`
					Title       string `json:"title"`
					Description string `json:"description"`
					EntryPoint  string `json:"entryPoint"`
				} `json:"config"`
				FileCount *int   `json:"fileCount"`
				SizeBytes *int64 `json:"sizeBytes"`
			} `json:"hmis"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}

	if err := client.DoQuery(ctx, query, nil, &res); err != nil {
		return err
	}
	if len(res.Errors) > 0 {
		return fmt.Errorf("GraphQL error: %s", res.Errors[0].Message)
	}

	if client.cfg.JSONMode {
		return printJSON(res.Data.Hmis)
	}

	if len(res.Data.Hmis) == 0 {
		fmt.Println("No HMI dashboards found on broker")
		return nil
	}

	fmt.Printf("%-18s %-22s %-16s %-8s %-12s %s\n", "NAME", "TITLE", "PATH", "MAIN", "NODE", "FILES")
	fmt.Println(strings.Repeat("-", 85))
	for _, h := range res.Data.Hmis {
		title := h.Config.Title
		if title == "" {
			title = "-"
		}
		pathStr := h.Config.UrlPath
		if pathStr == "" {
			pathStr = "/" + h.Name
		}
		isMainStr := "no"
		if h.Config.IsMain {
			isMainStr = "yes"
		}
		fileCountStr := "-"
		if h.FileCount != nil {
			fileCountStr = strconv.Itoa(*h.FileCount)
		}
		fmt.Printf("%-18s %-22s %-16s %-8s %-12s %s\n", h.Name, title, pathStr, isMainStr, h.NodeId, fileCountStr)
	}
	return nil
}

func runHmiRemove(ctx context.Context, client *Client, args []string) error {
	if len(args) < 1 || hasHelpFlag(args) {
		fmt.Println("Usage: mmq hmi remove <name1> [name2...]")
		fmt.Println("       mmq hmis remove <name1> [name2...]")
		fmt.Println()
		fmt.Println("Delete and remove one or more deployed HMI dashboards.")
		fmt.Println()
		fmt.Println("Arguments:")
		fmt.Println("  <name...>   One or more HMI dashboard names to delete (required)")
		fmt.Println()
		fmt.Println("Options:")
		fmt.Println("  -h, --help  Show this help text")
		return nil
	}

	query := `
		mutation DeleteHmi($name: String!) {
			hmi {
				delete(name: $name) {
					success
					message
				}
			}
		}
	`

	var results []map[string]any

	for _, name := range args {
		if strings.HasPrefix(name, "-") {
			continue
		}
		var res struct {
			Data struct {
				Hmi struct {
					Delete struct {
						Success bool   `json:"success"`
						Message string `json:"message"`
					} `json:"delete"`
				} `json:"hmi"`
			} `json:"data"`
			Errors []struct {
				Message string `json:"message"`
			} `json:"errors"`
		}

		if err := client.DoQuery(ctx, query, map[string]any{"name": name}, &res); err != nil {
			return err
		}
		if len(res.Errors) > 0 {
			return fmt.Errorf("GraphQL error: %s", res.Errors[0].Message)
		}

		delRes := res.Data.Hmi.Delete
		if client.cfg.JSONMode {
			results = append(results, map[string]any{
				"name":    name,
				"success": delRes.Success,
				"message": delRes.Message,
			})
		} else {
			if delRes.Success {
				fmt.Printf("✓ HMI dashboard '%s' deleted successfully\n", name)
			} else {
				errMsg := delRes.Message
				if errMsg == "" {
					errMsg = "failed to delete"
				}
				fmt.Printf("✗ Failed to delete HMI dashboard '%s': %s\n", name, errMsg)
			}
		}
	}

	if client.cfg.JSONMode {
		return printJSON(results)
	}
	return nil
}

func runHmiCreate(ctx context.Context, client *Client, args []string) error {
	if len(args) < 1 || hasHelpFlag(args) {
		fmt.Println("Usage: mmq hmi create <name> [options]")
		fmt.Println()
		fmt.Println("Create a new HMI dashboard definition.")
		fmt.Println()
		fmt.Println("Arguments:")
		fmt.Println("  <name>                   Unique HMI dashboard name (required)")
		fmt.Println()
		fmt.Println("Options:")
		fmt.Println("  --path <urlPath>         URL path (default: /<name>)")
		fmt.Println("  --title <title>          Display title for the dashboard (default: <name>)")
		fmt.Println("  --description <desc>     Description of the dashboard")
		fmt.Println("  --entry-point <file>     HTML entrypoint filename (default: index.html)")
		fmt.Println("  --main, -m               Designate as the primary/default dashboard (default: false)")
		fmt.Println("  --node <nodeId>          Target cluster node ID")
		fmt.Println("  --disabled               Create in disabled state (default: enabled)")
		fmt.Println("  -h, --help               Show this help text")
		return nil
	}

	name := args[0]
	urlPath := "/" + name
	title := name
	desc := ""
	entryPoint := "index.html"
	isMain := false
	enabled := true
	var nodeId *string

	for i := 1; i < len(args); i++ {
		arg := args[i]
		switch {
		case (arg == "--path" || arg == "-p") && i+1 < len(args):
			urlPath = args[i+1]
			if !strings.HasPrefix(urlPath, "/") {
				urlPath = "/" + urlPath
			}
			i++
		case (arg == "--title" || arg == "-t") && i+1 < len(args):
			title = args[i+1]
			i++
		case (arg == "--description" || arg == "-d") && i+1 < len(args):
			desc = args[i+1]
			i++
		case arg == "--entry-point" && i+1 < len(args):
			entryPoint = args[i+1]
			i++
		case arg == "--node" && i+1 < len(args):
			n := args[i+1]
			nodeId = &n
			i++
		case arg == "--main" || arg == "-m":
			isMain = true
		case arg == "--disabled":
			enabled = false
		}
	}

	input := map[string]any{
		"name":    name,
		"enabled": enabled,
		"config": map[string]any{
			"urlPath":     urlPath,
			"isMain":      isMain,
			"title":       title,
			"description": desc,
			"entryPoint":  entryPoint,
		},
	}
	if nodeId != nil {
		input["nodeId"] = *nodeId
	}

	query := `
		mutation CreateHmi($input: HmiInput!) {
			hmi {
				create(input: $input) {
					success
					message
					hmi {
						name
						nodeId
						enabled
						config {
							urlPath
							isMain
							title
							description
							entryPoint
						}
					}
				}
			}
		}
	`
	var res struct {
		Data struct {
			Hmi struct {
				Create struct {
					Success bool   `json:"success"`
					Message string `json:"message"`
					Hmi     *struct {
						Name    string `json:"name"`
						NodeId  string `json:"nodeId"`
						Enabled bool   `json:"enabled"`
						Config  struct {
							UrlPath     string `json:"urlPath"`
							IsMain      bool   `json:"isMain"`
							Title       string `json:"title"`
							Description string `json:"description"`
							EntryPoint  string `json:"entryPoint"`
						} `json:"config"`
					} `json:"hmi"`
				} `json:"create"`
			} `json:"hmi"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}

	if err := client.DoQuery(ctx, query, map[string]any{"input": input}, &res); err != nil {
		return err
	}
	if len(res.Errors) > 0 {
		return fmt.Errorf("GraphQL error: %s", res.Errors[0].Message)
	}

	createRes := res.Data.Hmi.Create
	if client.cfg.JSONMode {
		return printJSON(createRes)
	}

	if !createRes.Success {
		errMsg := createRes.Message
		if errMsg == "" {
			errMsg = "unknown error"
		}
		return fmt.Errorf("failed to create HMI dashboard: %s", errMsg)
	}

	fmt.Printf("✓ HMI dashboard '%s' created successfully (Path: %s, Title: %q)\n", name, urlPath, title)
	return nil
}

func runCurrentUser(ctx context.Context, client *Client, args []string) error {
	if hasHelpFlag(args) {
		fmt.Println("Usage: mmq currentUser")
		fmt.Println()
		fmt.Println("Inspect authenticated user and admin privileges.")
		return nil
	}
	query := `
		query {
			currentUser {
				username
				isAdmin
			}
		}
	`
	var res struct {
		Data struct {
			CurrentUser *struct {
				Username string `json:"username"`
				IsAdmin  bool   `json:"isAdmin"`
			} `json:"currentUser"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}

	if err := client.DoQuery(ctx, query, nil, &res); err != nil {
		return err
	}
	if len(res.Errors) > 0 {
		return fmt.Errorf("GraphQL error: %s", res.Errors[0].Message)
	}

	if client.cfg.JSONMode {
		return printJSON(res.Data.CurrentUser)
	}

	if res.Data.CurrentUser == nil {
		fmt.Println("Not authenticated (anonymous user)")
		return nil
	}

	fmt.Printf("User:  %s\n", res.Data.CurrentUser.Username)
	fmt.Printf("Admin: %v\n", res.Data.CurrentUser.IsAdmin)
	return nil
}

func runDatabaseConnections(ctx context.Context, client *Client, args []string) error {
	if hasHelpFlag(args) {
		fmt.Println("Usage: mmq databaseConnections [type]")
		fmt.Println()
		fmt.Println("List configured database connections (e.g. POSTGRES, MONGODB, SQLITE).")
		fmt.Println()
		fmt.Println("Arguments:")
		fmt.Println("  [type]                   Optional filter by database type (POSTGRES, MONGODB, SQLITE)")
		fmt.Println()
		fmt.Println("Options:")
		fmt.Println("  -h, --help               Show this help text")
		return nil
	}

	var connType *string
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		t := strings.ToUpper(args[0])
		connType = &t
	}

	query := `
		query DatabaseConnections($type: DatabaseConnectionType) {
			databaseConnections(type: $type) {
				name
				type
				url
				database
				schema
				readOnly
			}
		}
	`
	var res struct {
		Data struct {
			DatabaseConnections []struct {
				Name     string `json:"name"`
				Type     string `json:"type"`
				URL      string `json:"url"`
				Database string `json:"database"`
				Schema   string `json:"schema"`
				ReadOnly bool   `json:"readOnly"`
			} `json:"databaseConnections"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}

	vars := map[string]any{}
	if connType != nil {
		vars["type"] = *connType
	}

	if err := client.DoQuery(ctx, query, vars, &res); err != nil {
		return err
	}
	if len(res.Errors) > 0 {
		return fmt.Errorf("GraphQL error: %s", res.Errors[0].Message)
	}

	if client.cfg.JSONMode {
		return printJSON(res.Data.DatabaseConnections)
	}

	if len(res.Data.DatabaseConnections) == 0 {
		fmt.Println("No database connections found")
		return nil
	}

	fmt.Printf("%-24s %-12s %-10s %-20s %s\n", "NAME", "TYPE", "READONLY", "DATABASE", "URL")
	fmt.Println(strings.Repeat("-", 80))
	for _, db := range res.Data.DatabaseConnections {
		dbName := db.Database
		if dbName == "" {
			dbName = "-"
		}
		fmt.Printf("%-24s %-12s %-10t %-20s %s\n", db.Name, db.Type, db.ReadOnly, dbName, db.URL)
	}
	return nil
}

func runExportHmiZip(ctx context.Context, client *Client, args []string) error {
	if len(args) < 1 || hasHelpFlag(args) {
		fmt.Println("Usage: mmq exportHmiZip <dashboard-name> [output-file.zip] [options]")
		fmt.Println("       mmq exportHmiZip <dashboard-name> [target-directory] --unzip")
		fmt.Println()
		fmt.Println("Export a deployed HMI dashboard as a binary zip file or extract to a folder.")
		fmt.Println()
		fmt.Println("Arguments:")
		fmt.Println("  <dashboard-name>         Name of the deployed HMI dashboard (required)")
		fmt.Println("  [output-file.zip]        Output zip file path or target directory (default: <dashboard-name>.zip)")
		fmt.Println()
		fmt.Println("Options:")
		fmt.Println("  --unzip, -u              Extract and unzip dashboard files directly into a target folder")
		fmt.Println("  -h, --help               Show this help text")
		return nil
	}

	var name, outTarget string
	unzipMode := false

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--unzip" || arg == "-u" {
			unzipMode = true
		} else if !strings.HasPrefix(arg, "-") {
			if name == "" {
				name = arg
			} else if outTarget == "" {
				outTarget = arg
			}
		}
	}

	if name == "" {
		return fmt.Errorf("missing dashboard name")
	}

	if outTarget == "" {
		if unzipMode {
			outTarget = name
		} else {
			outTarget = name
			if !strings.HasSuffix(strings.ToLower(outTarget), ".zip") {
				outTarget += ".zip"
			}
		}
	}

	query := `
		query ExportHmiZip($name: String!) {
			exportHmiZip(name: $name)
		}
	`
	var res struct {
		Data struct {
			ExportHmiZip string `json:"exportHmiZip"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}

	if err := client.DoQuery(ctx, query, map[string]any{"name": name}, &res); err != nil {
		return err
	}
	if len(res.Errors) > 0 {
		return fmt.Errorf("GraphQL error: %s", res.Errors[0].Message)
	}

	rawB64 := res.Data.ExportHmiZip
	if rawB64 == "" {
		return fmt.Errorf("empty export data returned for HMI '%s'", name)
	}

	zipBytes, err := base64.StdEncoding.DecodeString(rawB64)
	if err != nil {
		return fmt.Errorf("failed to decode base64 zip data: %w", err)
	}

	if unzipMode {
		if err := unzipBytesToDir(zipBytes, outTarget); err != nil {
			return err
		}
		if client.cfg.JSONMode {
			return printJSON(map[string]any{
				"name":      name,
				"directory": outTarget,
				"unzipped":  true,
				"sizeBytes": len(zipBytes),
				"success":   true,
			})
		}
		fmt.Printf("✓ Exported and unzipped HMI dashboard '%s' into '%s/' (%d bytes archive)\n", name, outTarget, len(zipBytes))
		return nil
	}

	if err := os.WriteFile(outTarget, zipBytes, 0644); err != nil {
		return fmt.Errorf("error writing zip file '%s': %w", outTarget, err)
	}

	if client.cfg.JSONMode {
		return printJSON(map[string]any{
			"name":      name,
			"file":      outTarget,
			"sizeBytes": len(zipBytes),
			"success":   true,
		})
	}

	fmt.Printf("✓ Exported HMI dashboard '%s' to '%s' (%d bytes)\n", name, outTarget, len(zipBytes))
	return nil
}

func fileExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}

func isDir(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
}

func zipDirectory(sourceDir string) ([]byte, error) {
	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)

	err := filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}

		if relPath == "." {
			return nil
		}

		zipPath := filepath.ToSlash(relPath)

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = zipPath

		if info.IsDir() {
			header.Name += "/"
		} else {
			header.Method = zip.Deflate
		}

		w, err := zw.CreateHeader(header)
		if err != nil {
			return err
		}

		if !info.IsDir() {
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()
			if _, err := io.Copy(w, file); err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		_ = zw.Close()
		return nil, err
	}

	if err := zw.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func unzipBytesToDir(zipBytes []byte, destDir string) error {
	zr, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		return fmt.Errorf("invalid zip archive: %w", err)
	}

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory '%s': %w", destDir, err)
	}

	cleanDest := filepath.Clean(destDir)

	for _, f := range zr.File {
		targetPath := filepath.Join(destDir, filepath.Clean(f.Name))

		// Security: Prevent ZipSlip vulnerability
		if !strings.HasPrefix(targetPath, cleanDest+string(os.PathSeparator)) && targetPath != cleanDest {
			return fmt.Errorf("illegal file path in zip archive: %s", f.Name)
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(targetPath, f.Mode()); err != nil {
				return fmt.Errorf("failed to create directory '%s': %w", targetPath, err)
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return fmt.Errorf("failed to create parent directory for '%s': %w", targetPath, err)
		}

		rc, err := f.Open()
		if err != nil {
			return fmt.Errorf("failed to read file '%s' from zip: %w", f.Name, err)
		}

		outFile, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			_ = rc.Close()
			return fmt.Errorf("failed to create output file '%s': %w", targetPath, err)
		}

		_, copyErr := io.Copy(outFile, rc)
		_ = rc.Close()
		_ = outFile.Close()
		if copyErr != nil {
			return fmt.Errorf("failed to extract file '%s': %w", targetPath, copyErr)
		}
	}
	return nil
}

func runImportHmiZip(ctx context.Context, client *Client, args []string) error {
	if len(args) < 1 || hasHelpFlag(args) {
		fmt.Println("Usage: mmq importHmiZip <file.zip|directory> [dashboard-name] [options]")
		fmt.Println("       mmq importHmiZip <dashboard-name> <file.zip|directory> [options]")
		fmt.Println()
		fmt.Println("Upload and deploy an HMI web dashboard from a binary zip file or local directory.")
		fmt.Println()
		fmt.Println("Arguments:")
		fmt.Println("  <file.zip|directory>     Path to the .zip file package or folder to auto-zip (required)")
		fmt.Println("  [dashboard-name]         Name for the HMI dashboard (defaults to zip/folder name)")
		fmt.Println()
		fmt.Println("Options:")
		fmt.Println("  --main, -m               Set this dashboard as the primary / default HMI")
		fmt.Println("  -h, --help               Show this help text")
		return nil
	}

	var sourceArg, nameArg string
	var setAsMain *bool

	posArgs := []string{}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--main" || arg == "-m" {
			v := true
			setAsMain = &v
		} else if !strings.HasPrefix(arg, "-") {
			posArgs = append(posArgs, arg)
		}
	}

	if len(posArgs) < 1 {
		return fmt.Errorf("missing zip file or dashboard directory")
	}

	if strings.HasSuffix(strings.ToLower(posArgs[0]), ".zip") || fileExists(posArgs[0]) || isDir(posArgs[0]) {
		sourceArg = posArgs[0]
		if len(posArgs) > 1 {
			nameArg = posArgs[1]
		}
	} else if len(posArgs) > 1 && (strings.HasSuffix(strings.ToLower(posArgs[1]), ".zip") || fileExists(posArgs[1]) || isDir(posArgs[1])) {
		nameArg = posArgs[0]
		sourceArg = posArgs[1]
	} else {
		sourceArg = posArgs[0]
		if len(posArgs) > 1 {
			nameArg = posArgs[1]
		}
	}

	if nameArg == "" {
		base := filepath.Base(sourceArg)
		nameArg = strings.TrimSuffix(base, filepath.Ext(base))
	}

	var zipBytes []byte
	var err error
	var sourceDesc string

	if isDir(sourceArg) {
		sourceDesc = fmt.Sprintf("directory '%s' (auto-zipped)", sourceArg)
		zipBytes, err = zipDirectory(sourceArg)
		if err != nil {
			return fmt.Errorf("failed to zip directory '%s': %w", sourceArg, err)
		}
	} else {
		sourceDesc = fmt.Sprintf("file '%s'", sourceArg)
		zipBytes, err = os.ReadFile(sourceArg)
		if err != nil {
			return fmt.Errorf("failed to read file '%s': %w", sourceArg, err)
		}
	}

	zipB64 := base64.StdEncoding.EncodeToString(zipBytes)

	query := `
		mutation UploadHmiZip($name: String!, $zipBase64: String!, $setAsMain: Boolean) {
			hmi {
				uploadZip(name: $name, zipBase64: $zipBase64, setAsMain: $setAsMain) {
					success
					message
					hmi {
						name
						nodeId
						enabled
						config {
							urlPath
							isMain
							title
						}
					}
				}
			}
		}
	`
	vars := map[string]any{
		"name":      nameArg,
		"zipBase64": zipB64,
	}
	if setAsMain != nil {
		vars["setAsMain"] = *setAsMain
	}

	var res struct {
		Data struct {
			Hmi struct {
				UploadZip struct {
					Success bool   `json:"success"`
					Message string `json:"message"`
					Hmi     *struct {
						Name    string `json:"name"`
						NodeId  string `json:"nodeId"`
						Enabled bool   `json:"enabled"`
						Config  struct {
							UrlPath string `json:"urlPath"`
							IsMain  bool   `json:"isMain"`
							Title   string `json:"title"`
						} `json:"config"`
					} `json:"hmi"`
				} `json:"uploadZip"`
			} `json:"hmi"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}

	if err := client.DoQuery(ctx, query, vars, &res); err != nil {
		return err
	}
	if len(res.Errors) > 0 {
		return fmt.Errorf("GraphQL error: %s", res.Errors[0].Message)
	}

	result := res.Data.Hmi.UploadZip
	if client.cfg.JSONMode {
		return printJSON(result)
	}

	if !result.Success {
		errMsg := result.Message
		if errMsg == "" {
			errMsg = "unknown error"
		}
		return fmt.Errorf("HMI upload failed: %s", errMsg)
	}

	urlPath := ""
	if result.Hmi != nil && result.Hmi.Config.UrlPath != "" {
		urlPath = fmt.Sprintf(" (URL: %s)", result.Hmi.Config.UrlPath)
	}
	fmt.Printf("✓ HMI dashboard '%s' imported successfully from %s%s\n", nameArg, sourceDesc, urlPath)
	return nil
}

