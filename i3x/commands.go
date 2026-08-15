package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// CommandHandler dispatches CLI and REPL commands.
type CommandHandler struct {
	client    *Client
	formatter *Formatter
}

// NewCommandHandler creates a new CommandHandler instance.
func NewCommandHandler(client *Client, formatter *Formatter) *CommandHandler {
	return &CommandHandler{
		client:    client,
		formatter: formatter,
	}
}

// Execute executes a command line with arguments.
func (h *CommandHandler) Execute(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("no command specified. Run 'i3x --help' for usage")
	}

	cmd := strings.ToLower(args[0])
	cmdArgs := args[1:]

	switch cmd {
	case "info", "health", "ping":
		return h.CmdInfo(ctx, cmdArgs)

	case "namespaces", "ns":
		return h.CmdNamespaces(ctx, cmdArgs)

	case "types", "objecttypes", "object-types":
		return h.CmdObjectTypes(ctx, cmdArgs)

	case "type-query", "types-query", "object-types-query":
		return h.CmdObjectTypesQuery(ctx, cmdArgs)

	case "rel-types", "relationshiptypes", "relationship-types":
		return h.CmdRelationshipTypes(ctx, cmdArgs)

	case "rel-types-query", "relationshiptypes-query":
		return h.CmdRelationshipTypesQuery(ctx, cmdArgs)

	case "objects", "obj":
		return h.CmdObjects(ctx, cmdArgs)

	case "objects-list", "objects-query", "object-list":
		return h.CmdObjectsList(ctx, cmdArgs)

	case "related", "objects-related":
		return h.CmdObjectsRelated(ctx, cmdArgs)

	case "read", "get-value", "value", "values":
		return h.CmdReadValue(ctx, cmdArgs)

	case "write", "set-value":
		return h.CmdWriteValue(ctx, cmdArgs)

	case "history", "get-history":
		return h.CmdGetHistory(ctx, cmdArgs)

	case "write-history", "set-history":
		return h.CmdWriteHistory(ctx, cmdArgs)

	case "sub", "subscription", "subscriptions":
		return h.CmdSubscription(ctx, cmdArgs)

	case "watch", "subscribe", "listen":
		return h.CmdWatch(ctx, cmdArgs)

	case "help", "--help", "-h":
		PrintHelp()
		return nil

	default:
		return fmt.Errorf("unknown command %q. Run 'i3x --help' for available commands", cmd)
	}
}

func hasHelpFlag(args []string) bool {
	for _, a := range args {
		if a == "-h" || a == "--help" || a == "help" {
			return true
		}
	}
	return false
}

// -------------------------------------------------------------
// 1. Info & Health
// -------------------------------------------------------------

func (h *CommandHandler) CmdInfo(ctx context.Context, args []string) error {
	if hasHelpFlag(args) {
		fmt.Println("Usage: i3x info")
		fmt.Println()
		fmt.Println("Display i3X server capabilities, spec version, and server information.")
		return nil
	}
	info, err := h.client.GetInfo(ctx)
	if err != nil {
		return fmt.Errorf("failed to get server info: %w", err)
	}
	h.formatter.PrintServerInfo(info)
	return nil
}

// -------------------------------------------------------------
// 2. Namespaces
// -------------------------------------------------------------

func (h *CommandHandler) CmdNamespaces(ctx context.Context, args []string) error {
	if hasHelpFlag(args) {
		fmt.Println("Usage: i3x namespaces")
		fmt.Println()
		fmt.Println("List all registered namespaces in the i3X server address space.")
		return nil
	}
	ns, err := h.client.GetNamespaces(ctx)
	if err != nil {
		return fmt.Errorf("failed to get namespaces: %w", err)
	}
	h.formatter.PrintNamespaces(ns)
	return nil
}

// -------------------------------------------------------------
// 3. Object Types
// -------------------------------------------------------------

func (h *CommandHandler) CmdObjectTypes(ctx context.Context, args []string) error {
	if hasHelpFlag(args) {
		fmt.Println("Usage: i3x types [options]")
		fmt.Println("       i3x types query <elementId...>")
		fmt.Println()
		fmt.Println("List or query i3X object type schema definitions.")
		fmt.Println()
		fmt.Println("Options:")
		fmt.Println("  --namespace <uri>, -n <uri> Filter by namespace URI (GET /v1/objecttypes?namespaceUri=...)")
		fmt.Println("  -h, --help                  Show this help text")
		return nil
	}

	if len(args) > 0 && (args[0] == "query" || args[0] == "get") {
		return h.CmdObjectTypesQuery(ctx, args[1:])
	}

	namespaceURI := ""
	for i := 0; i < len(args); i++ {
		if (args[i] == "--namespace" || args[i] == "-n" || args[i] == "--ns") && i+1 < len(args) {
			namespaceURI = args[i+1]
			i++
		} else if !strings.HasPrefix(args[i], "-") {
			namespaceURI = args[i]
		}
	}

	types, err := h.client.GetObjectTypes(ctx, namespaceURI)
	if err != nil {
		return fmt.Errorf("failed to get object types: %w", err)
	}
	h.formatter.PrintObjectTypes(types)
	return nil
}

func (h *CommandHandler) CmdObjectTypesQuery(ctx context.Context, args []string) error {
	if hasHelpFlag(args) || len(args) == 0 {
		fmt.Println("Usage: i3x types query <elementId...>")
		fmt.Println()
		fmt.Println("Query specific object type schema definitions by element ID (POST /v1/objecttypes/query).")
		return nil
	}
	var elementIDs []string
	for _, a := range args {
		if !strings.HasPrefix(a, "-") {
			elementIDs = append(elementIDs, a)
		}
	}
	res, err := h.client.QueryObjectTypes(ctx, elementIDs)
	if err != nil {
		return fmt.Errorf("failed to query object types: %w", err)
	}

	if h.formatter.Format == FormatJSON {
		h.formatter.PrintJSON(res)
		return nil
	}
	if h.formatter.Format == FormatRaw {
		h.formatter.PrintRaw(res)
		return nil
	}

	var types []ObjectTypeResponse
	for _, item := range res.Results {
		if item.Success && item.Result != nil {
			types = append(types, *item.Result)
		} else {
			id := "-"
			if item.ElementID != nil {
				id = *item.ElementID
			}
			errStr := "not found"
			if item.ResponseDetail != nil {
				errStr = item.ResponseDetail.Error()
			}
			fmt.Fprintf(h.formatter.Out, "%s: %s\n", h.formatter.color(colorRed, id), errStr)
		}
	}
	if len(types) > 0 {
		h.formatter.PrintObjectTypes(types)
	}
	return nil
}

// -------------------------------------------------------------
// 4. Relationship Types
// -------------------------------------------------------------

func (h *CommandHandler) CmdRelationshipTypes(ctx context.Context, args []string) error {
	if hasHelpFlag(args) {
		fmt.Println("Usage: i3x rel-types [options]")
		fmt.Println("       i3x rel-types query <elementId...>")
		fmt.Println()
		fmt.Println("List or query i3X relationship type definitions.")
		fmt.Println()
		fmt.Println("Options:")
		fmt.Println("  --namespace <uri>, -n <uri> Filter by namespace URI (GET /v1/relationshiptypes?namespaceUri=...)")
		fmt.Println("  -h, --help                  Show this help text")
		return nil
	}

	if len(args) > 0 && (args[0] == "query" || args[0] == "get") {
		return h.CmdRelationshipTypesQuery(ctx, args[1:])
	}

	namespaceURI := ""
	for i := 0; i < len(args); i++ {
		if (args[i] == "--namespace" || args[i] == "-n" || args[i] == "--ns") && i+1 < len(args) {
			namespaceURI = args[i+1]
			i++
		} else if !strings.HasPrefix(args[i], "-") {
			namespaceURI = args[i]
		}
	}

	relTypes, err := h.client.GetRelationshipTypes(ctx, namespaceURI)
	if err != nil {
		return fmt.Errorf("failed to get relationship types: %w", err)
	}
	h.formatter.PrintRelationshipTypes(relTypes)
	return nil
}

func (h *CommandHandler) CmdRelationshipTypesQuery(ctx context.Context, args []string) error {
	if hasHelpFlag(args) || len(args) == 0 {
		fmt.Println("Usage: i3x rel-types query <elementId...>")
		fmt.Println()
		fmt.Println("Query specific relationship type definitions by element ID (POST /v1/relationshiptypes/query).")
		return nil
	}
	var elementIDs []string
	for _, a := range args {
		if !strings.HasPrefix(a, "-") {
			elementIDs = append(elementIDs, a)
		}
	}
	res, err := h.client.QueryRelationshipTypes(ctx, elementIDs)
	if err != nil {
		return fmt.Errorf("failed to query relationship types: %w", err)
	}

	if h.formatter.Format == FormatJSON {
		h.formatter.PrintJSON(res)
		return nil
	}
	if h.formatter.Format == FormatRaw {
		h.formatter.PrintRaw(res)
		return nil
	}

	var relTypes []RelationshipType
	for _, item := range res.Results {
		if item.Success && item.Result != nil {
			relTypes = append(relTypes, *item.Result)
		} else {
			id := "-"
			if item.ElementID != nil {
				id = *item.ElementID
			}
			errStr := "not found"
			if item.ResponseDetail != nil {
				errStr = item.ResponseDetail.Error()
			}
			fmt.Fprintf(h.formatter.Out, "%s: %s\n", h.formatter.color(colorRed, id), errStr)
		}
	}
	if len(relTypes) > 0 {
		h.formatter.PrintRelationshipTypes(relTypes)
	}
	return nil
}

// -------------------------------------------------------------
// 5. Objects
// -------------------------------------------------------------

func matchFilter(val, pattern string) bool {
	if pattern == "" || pattern == "*" {
		return true
	}
	valLower := strings.ToLower(val)
	patLower := strings.ToLower(pattern)

	if strings.ContainsAny(pattern, "*?[") {
		matched, err := filepath.Match(patLower, valLower)
		if err == nil && matched {
			return true
		}
	}
	return strings.Contains(valLower, patLower)
}

func (h *CommandHandler) CmdObjects(ctx context.Context, args []string) error {
	if len(args) > 0 {
		sub := strings.ToLower(args[0])
		if sub == "query" || sub == "get" || sub == "list-by-id" {
			return h.CmdObjectsList(ctx, args[1:])
		}
		if sub == "related" {
			return h.CmdObjectsRelated(ctx, args[1:])
		}
		if sub == "-h" || sub == "--help" || sub == "help" {
			fmt.Println("Usage: i3x objects [options]")
			fmt.Println("       i3x objects query <id...> [--metadata]")
			fmt.Println("       i3x objects related <id...> [--rel-type <type>]")
			fmt.Println()
			fmt.Println("List, filter, and inspect i3X object instances.")
			fmt.Println()
			fmt.Println("Filter Options:")
			fmt.Println("  --type <id>, -t <id>     Filter by ObjectType element ID (GET /v1/objects?typeElementId=...)")
			fmt.Println("  --filter <pat>, -f <pat> Filter by element ID, name, or type (substring or wildcard *pattern*)")
			fmt.Println("  --name <pat>             Filter specifically by display name")
			fmt.Println("  --parent <parentId>      Filter objects by parent element ID")
			fmt.Println("  --root                   Filter to root objects only (GET /v1/objects?root=true)")
			fmt.Println("  --non-root               Filter to non-root objects only (GET /v1/objects?root=false)")
			fmt.Println("  --composition            Filter to composition objects only (isComposition=true)")
			fmt.Println("  --non-composition        Filter to non-composition objects only (isComposition=false)")
			fmt.Println("  --metadata, -m           Include full metadata and attributes")
			fmt.Println("  --format <fmt>, -o <fmt> Output format: table, tree, json, csv, raw")
			fmt.Println("  -h, --help               Show this help text")
			return nil
		}
	}

	typeElementID := ""
	filterPattern := ""
	namePattern := ""
	parentIDFilter := ""
	var compositionFilter *bool
	includeMetadata := false
	var root *bool

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-h" || arg == "--help":
			fmt.Println("Usage: i3x objects [options]")
			fmt.Println()
			fmt.Println("List and filter i3X object instances.")
			fmt.Println()
			fmt.Println("Filter Options:")
			fmt.Println("  --type <id>, -t <id>     Filter by ObjectType element ID (GET /v1/objects?typeElementId=...)")
			fmt.Println("  --filter <pat>, -f <pat> Filter by element ID, name, or type (substring or wildcard *pattern*)")
			fmt.Println("  --name <pat>             Filter specifically by display name")
			fmt.Println("  --parent <parentId>      Filter objects by parent element ID")
			fmt.Println("  --root                   Filter to root objects only")
			fmt.Println("  --non-root               Filter to non-root objects only")
			fmt.Println("  --composition            Filter to composition objects only")
			fmt.Println("  --non-composition        Filter to non-composition objects only")
			fmt.Println("  --metadata, -m           Include full metadata and attributes")
			fmt.Println("  --format <fmt>, -o <fmt> Output format: table, tree, json, csv, raw")
			fmt.Println("  -h, --help               Show this help text")
			return nil
		case (arg == "--type" || arg == "-t") && i+1 < len(args):
			typeElementID = args[i+1]
			i++
		case (arg == "--filter" || arg == "-f" || arg == "--search" || arg == "-s") && i+1 < len(args):
			filterPattern = args[i+1]
			i++
		case arg == "--name" && i+1 < len(args):
			namePattern = args[i+1]
			i++
		case arg == "--parent" && i+1 < len(args):
			parentIDFilter = args[i+1]
			i++
		case arg == "--composition":
			val := true
			compositionFilter = &val
		case arg == "--non-composition":
			val := false
			compositionFilter = &val
		case arg == "--metadata" || arg == "-m" || arg == "--include-metadata" || arg == "--includeMetadata" || arg == "--includeMetadata=true" || arg == "--include-metadata=true":
			includeMetadata = true
		case arg == "--includeMetadata=false" || arg == "--include-metadata=false":
			includeMetadata = false
		case arg == "--root" || arg == "--root=true":
			val := true
			root = &val
		case arg == "--non-root" || arg == "--root=false":
			val := false
			root = &val
		case (arg == "--format" || arg == "-o") && i+1 < len(args):
			h.formatter.SetFormat(args[i+1])
			i++
		default:
			if !strings.HasPrefix(arg, "-") && typeElementID == "" && filterPattern == "" {
				typeElementID = arg
			}
		}
	}

	objects, err := h.client.GetObjects(ctx, typeElementID, includeMetadata, root)
	if err != nil {
		return fmt.Errorf("failed to get objects: %w", err)
	}

	// Apply additional filters
	if filterPattern != "" || namePattern != "" || parentIDFilter != "" || compositionFilter != nil {
		var filtered []ObjectInstanceResponse
		for _, o := range objects {
			if filterPattern != "" {
				if !matchFilter(o.ElementID, filterPattern) && !matchFilter(o.DisplayName, filterPattern) && !matchFilter(o.TypeElementID, filterPattern) {
					continue
				}
			}
			if namePattern != "" {
				if !matchFilter(o.DisplayName, namePattern) {
					continue
				}
			}
			if parentIDFilter != "" {
				if o.ParentID == nil || !matchFilter(*o.ParentID, parentIDFilter) {
					continue
				}
			}
			if compositionFilter != nil {
				if o.IsComposition != *compositionFilter {
					continue
				}
			}
			filtered = append(filtered, o)
		}
		objects = filtered
	}

	h.formatter.PrintObjects(objects)
	return nil
}

func (h *CommandHandler) CmdObjectsList(ctx context.Context, args []string) error {
	if hasHelpFlag(args) || len(args) == 0 {
		fmt.Println("Usage: i3x objects query <elementId...> [options]")
		fmt.Println()
		fmt.Println("Query specific object instances by element ID (POST /v1/objects/list).")
		fmt.Println()
		fmt.Println("Options:")
		fmt.Println("  --metadata, -m           Include full metadata and attributes")
		fmt.Println("  -h, --help               Show this help text")
		return nil
	}
	var elementIDs []string
	includeMetadata := false

	for _, a := range args {
		if a == "--metadata" || a == "-m" || a == "--include-metadata" || a == "--includeMetadata" {
			includeMetadata = true
		} else if !strings.HasPrefix(a, "-") {
			elementIDs = append(elementIDs, a)
		}
	}

	if len(elementIDs) == 0 {
		return errors.New("no element IDs provided")
	}

	res, err := h.client.ListObjects(ctx, elementIDs, includeMetadata)
	if err != nil {
		return fmt.Errorf("failed to query objects by ID: %w", err)
	}

	if h.formatter.Format == FormatJSON {
		h.formatter.PrintJSON(res)
		return nil
	}
	if h.formatter.Format == FormatRaw {
		h.formatter.PrintRaw(res)
		return nil
	}

	var objects []ObjectInstanceResponse
	for _, item := range res.Results {
		if item.Success && item.Result != nil {
			objects = append(objects, *item.Result)
		} else {
			id := "-"
			if item.ElementID != nil {
				id = *item.ElementID
			}
			errStr := "not found"
			if item.ResponseDetail != nil {
				errStr = item.ResponseDetail.Error()
			}
			fmt.Fprintf(h.formatter.Out, "%s: %s\n", h.formatter.color(colorRed, id), errStr)
		}
	}
	if len(objects) > 0 {
		h.formatter.PrintObjects(objects)
	}
	return nil
}

func (h *CommandHandler) CmdObjectsRelated(ctx context.Context, args []string) error {
	if hasHelpFlag(args) || len(args) == 0 {
		fmt.Println("Usage: i3x related <elementId...> [options]")
		fmt.Println()
		fmt.Println("Query related objects connected by relationship types (POST /v1/objects/related).")
		fmt.Println()
		fmt.Println("Options:")
		fmt.Println("  --rel-type <type>, -r <type> Filter by relationship type")
		fmt.Println("  --includeMetadata, -m        Include full metadata")
		fmt.Println("  -h, --help                   Show this help text")
		return nil
	}

	var elementIDs []string
	relType := ""
	includeMetadata := false

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--rel-type", "-r", "--relationship":
			if i+1 < len(args) {
				relType = args[i+1]
				i++
			}
		case "--metadata", "-m", "--include-metadata", "--includeMetadata":
			includeMetadata = true
		default:
			if !strings.HasPrefix(args[i], "-") {
				elementIDs = append(elementIDs, args[i])
			}
		}
	}

	if len(elementIDs) == 0 {
		return errors.New("no element IDs specified")
	}

	res, err := h.client.QueryRelatedObjects(ctx, elementIDs, relType, includeMetadata)
	if err != nil {
		return fmt.Errorf("failed to query related objects: %w", err)
	}

	if h.formatter.Format == FormatJSON {
		h.formatter.PrintJSON(res)
		return nil
	}
	if h.formatter.Format == FormatRaw {
		h.formatter.PrintRaw(res)
		return nil
	}

	for _, item := range res.Results {
		id := "-"
		if item.ElementID != nil {
			id = *item.ElementID
		}
		if !item.Success || item.Result == nil {
			errStr := "not found"
			if item.ResponseDetail != nil {
				errStr = item.ResponseDetail.Error()
			}
			fmt.Fprintf(h.formatter.Out, "%s: %s\n", h.formatter.color(colorRed, id), errStr)
			continue
		}

		fmt.Fprintf(h.formatter.Out, "%s: %d related object(s)\n", h.formatter.color(colorCyan+colorBold, id), len(*item.Result))
		for _, rel := range *item.Result {
			fmt.Fprintf(h.formatter.Out, "  [%s] -> %s (%s, type=%s)\n",
				h.formatter.color(colorYellow, rel.SourceRelationship),
				h.formatter.color(colorBold, rel.Object.ElementID),
				rel.Object.DisplayName,
				rel.Object.TypeElementID)
		}
	}
	return nil
}

// -------------------------------------------------------------
// 6. Value Queries & Writes
// -------------------------------------------------------------

func (h *CommandHandler) CmdReadValue(ctx context.Context, args []string) error {
	if hasHelpFlag(args) || len(args) == 0 {
		fmt.Println("Usage: i3x read <elementId...> [options]")
		fmt.Println()
		fmt.Println("Read last known current values of object instances (POST /v1/objects/value).")
		fmt.Println()
		fmt.Println("Options:")
		fmt.Println("  --depth <n>, -d <n> Maximum composition depth (depth=0 recurses all children)")
		fmt.Println("  -h, --help          Show this help text")
		return nil
	}

	var elementIDs []string
	maxDepth := 1

	for i := 0; i < len(args); i++ {
		if (args[i] == "--depth" || args[i] == "-d") && i+1 < len(args) {
			if n, err := strconv.Atoi(args[i+1]); err == nil {
				maxDepth = n
			}
			i++
		} else if !strings.HasPrefix(args[i], "-") {
			elementIDs = append(elementIDs, args[i])
		}
	}

	if len(elementIDs) == 0 {
		return errors.New("no element IDs provided")
	}

	bulk, err := h.client.QueryLastKnownValues(ctx, elementIDs, maxDepth)
	if err != nil {
		return fmt.Errorf("failed to query values: %w", err)
	}

	h.formatter.PrintCurrentValues(bulk)
	return nil
}

func (h *CommandHandler) CmdWriteValue(ctx context.Context, args []string) error {
	if hasHelpFlag(args) || len(args) == 0 {
		fmt.Println("Usage: i3x write <elementId> <value> [options]")
		fmt.Println("       i3x write id1=val1 id2=val2")
		fmt.Println("       i3x write --json '<json>'")
		fmt.Println()
		fmt.Println("Write current value(s) for object instances (PUT /v1/objects/value).")
		fmt.Println()
		fmt.Println("Options:")
		fmt.Println("  --quality <q>, -q <q>     Quality string (default: Good)")
		fmt.Println("  --timestamp <t>, -t <t>   RFC3339 timestamp or relative offset (e.g. -5m, now)")
		fmt.Println("  --json '<json>'           Raw JSON update payload")
		fmt.Println("  -h, --help                Show this help text")
		return nil
	}

	var updates []ValueUpdateItem

	if args[0] == "--json" && len(args) > 1 {
		raw := strings.Join(args[1:], " ")
		if err := json.Unmarshal([]byte(raw), &updates); err != nil {
			return fmt.Errorf("invalid json updates payload: %w", err)
		}
	} else if len(args) >= 2 && !strings.Contains(args[0], "=") {
		elementID := args[0]
		rawVal := args[1]
		val := ParseValueHelper(rawVal)

		quality := "Good"
		var timestamp *string

		for i := 2; i < len(args); i++ {
			if (args[i] == "--quality" || args[i] == "-q") && i+1 < len(args) {
				quality = args[i+1]
				i++
			} else if (args[i] == "--timestamp" || args[i] == "-t") && i+1 < len(args) {
				ts := parseOrFormatTimestamp(args[i+1])
				timestamp = &ts
				i++
			}
		}

		if timestamp == nil {
			ts := FormatTimeRFC3339(time.Now())
			timestamp = &ts
		}

		updates = append(updates, ValueUpdateItem{
			ElementID: elementID,
			Value: VQTInput{
				Value:     val,
				Quality:   &quality,
				Timestamp: timestamp,
			},
		})
	} else {
		// Key-value pairs: id1=val1 id2=val2
		for _, arg := range args {
			parts := strings.SplitN(arg, "=", 2)
			if len(parts) == 2 {
				val := ParseValueHelper(parts[1])
				q := "Good"
				ts := FormatTimeRFC3339(time.Now())
				updates = append(updates, ValueUpdateItem{
					ElementID: parts[0],
					Value: VQTInput{
						Value:     val,
						Quality:   &q,
						Timestamp: &ts,
					},
				})
			}
		}
	}

	if len(updates) == 0 {
		return errors.New("no valid element updates specified")
	}

	bulk, err := h.client.UpdateObjectValues(ctx, updates)
	if err != nil {
		return fmt.Errorf("failed to write object values: %w", err)
	}

	h.formatter.PrintBulkGeneric(bulk, "Value updated", "Failed to update value")
	return nil
}

// -------------------------------------------------------------
// 7. History Queries & Updates
// -------------------------------------------------------------

func (h *CommandHandler) CmdGetHistory(ctx context.Context, args []string) error {
	if hasHelpFlag(args) || len(args) == 0 {
		fmt.Println("Usage: i3x history <elementId...> [options]")
		fmt.Println()
		fmt.Println("Query historical time-series values for object instances (POST /v1/objects/history).")
		fmt.Println()
		fmt.Println("Options:")
		fmt.Println("  --start <time>, -s <time> Start time RFC3339 or relative duration (default: -1h)")
		fmt.Println("  --end <time>, -e <time>   End time RFC3339 or relative duration (default: now)")
		fmt.Println("  --depth <n>, -d <n>       Maximum composition depth (default: 1)")
		fmt.Println("  -h, --help                Show this help text")
		return nil
	}

	var elementIDs []string
	startTime := FormatTimeRFC3339(time.Now().Add(-1 * time.Hour))
	endTime := FormatTimeRFC3339(time.Now())
	maxDepth := 1

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--start", "-s":
			if i+1 < len(args) {
				startTime = parseOrFormatTimestamp(args[i+1])
				i++
			}
		case "--end", "-e":
			if i+1 < len(args) {
				endTime = parseOrFormatTimestamp(args[i+1])
				i++
			}
		case "--depth", "-d":
			if i+1 < len(args) {
				if n, err := strconv.Atoi(args[i+1]); err == nil {
					maxDepth = n
				}
				i++
			}
		default:
			if !strings.HasPrefix(args[i], "-") {
				elementIDs = append(elementIDs, args[i])
			}
		}
	}

	if len(elementIDs) == 0 {
		return errors.New("no element IDs specified")
	}

	bulk, err := h.client.QueryHistoricalValues(ctx, elementIDs, startTime, endTime, maxDepth)
	if err != nil {
		return fmt.Errorf("failed to query history: %w", err)
	}

	h.formatter.PrintHistoricalValues(bulk)
	return nil
}

func (h *CommandHandler) CmdWriteHistory(ctx context.Context, args []string) error {
	if hasHelpFlag(args) || len(args) == 0 {
		fmt.Println("Usage: i3x write-history <elementId> <value> [options]")
		fmt.Println("       i3x write-history --json '<json>'")
		fmt.Println()
		fmt.Println("Record historical data points (PUT /v1/objects/history).")
		fmt.Println()
		fmt.Println("Options:")
		fmt.Println("  --quality <q>, -q <q>     Quality string (default: Good)")
		fmt.Println("  --timestamp <t>, -t <t>   Timestamp for the entry (RFC3339 or relative offset)")
		fmt.Println("  --json '<json>'           Raw JSON update payload")
		fmt.Println("  -h, --help                Show this help text")
		return nil
	}

	var updates []HistoryUpdateItem

	if args[0] == "--json" && len(args) > 1 {
		raw := strings.Join(args[1:], " ")
		if err := json.Unmarshal([]byte(raw), &updates); err != nil {
			return fmt.Errorf("invalid json history updates payload: %w", err)
		}
	} else if len(args) >= 2 {
		elementID := args[0]
		rawVal := args[1]
		val := ParseValueHelper(rawVal)

		quality := "Good"
		ts := FormatTimeRFC3339(time.Now())

		for i := 2; i < len(args); i++ {
			if (args[i] == "--quality" || args[i] == "-q") && i+1 < len(args) {
				quality = args[i+1]
				i++
			} else if (args[i] == "--timestamp" || args[i] == "-t") && i+1 < len(args) {
				ts = parseOrFormatTimestamp(args[i+1])
				i++
			}
		}

		updates = append(updates, HistoryUpdateItem{
			ElementID: elementID,
			Value: VQT{
				Value:     val,
				Quality:   quality,
				Timestamp: ts,
			},
		})
	}

	if len(updates) == 0 {
		return errors.New("no history updates provided")
	}

	bulk, err := h.client.UpdateObjectHistory(ctx, updates)
	if err != nil {
		return fmt.Errorf("failed to write object history: %w", err)
	}

	h.formatter.PrintBulkGeneric(bulk, "Historical record added", "Failed to write history")
	return nil
}

// -------------------------------------------------------------
// 8. Subscriptions
// -------------------------------------------------------------

func (h *CommandHandler) CmdSubscription(ctx context.Context, args []string) error {
	if len(args) == 0 || (len(args) == 1 && hasHelpFlag(args)) {
		fmt.Println("Usage: i3x sub <command> [options]")
		fmt.Println()
		fmt.Println("Manage real-time telemetry subscriptions and SSE streams.")
		fmt.Println()
		fmt.Println("Commands:")
		fmt.Println("  sub create [--name <n>] [--client-id <id>]       Create a new subscription")
		fmt.Println("  sub list <subId...> [--client-id <id>]           List active subscriptions")
		fmt.Println("  sub register <subId> <id...> [--depth <n>]       Register objects to monitor")
		fmt.Println("  sub unregister <subId> <id...>                   Unregister objects from subscription")
		fmt.Println("  sub sync <subId> [--ack-seq <n>]                 Poll pending telemetry updates")
		fmt.Println("  sub stream <subId> [--client-id <id>]            Stream updates via SSE in real-time")
		fmt.Println("  sub delete <subId...> [--client-id <id>]         Delete subscription(s)")
		fmt.Println()
		fmt.Println("Options:")
		fmt.Println("  -h, --help                                       Show this help text")
		return nil
	}

	subCmd := strings.ToLower(args[0])
	subArgs := args[1:]

	switch subCmd {
	case "create", "new":
		return h.CmdSubCreate(ctx, subArgs)
	case "list", "ls", "get":
		return h.CmdSubList(ctx, subArgs)
	case "register", "reg", "add":
		return h.CmdSubRegister(ctx, subArgs)
	case "unregister", "unreg", "remove", "rm":
		return h.CmdSubUnregister(ctx, subArgs)
	case "sync", "pull":
		return h.CmdSubSync(ctx, subArgs)
	case "stream", "listen":
		return h.CmdSubStream(ctx, subArgs)
	case "delete", "del", "destroy":
		return h.CmdSubDelete(ctx, subArgs)
	default:
		return fmt.Errorf("unknown subscription subcommand %q. Run 'i3x sub --help' for available commands", subCmd)
	}
}

func (h *CommandHandler) CmdSubCreate(ctx context.Context, args []string) error {
	if hasHelpFlag(args) {
		fmt.Println("Usage: i3x sub create [name] [--name <displayName>] [--client-id <id>]")
		fmt.Println()
		fmt.Println("Create a new telemetry subscription (POST /v1/subscriptions).")
		return nil
	}
	name := ""
	clientID := ""

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--name", "-n", "--display-name":
			if i+1 < len(args) {
				name = args[i+1]
				i++
			}
		case "--client-id", "-c":
			if i+1 < len(args) {
				clientID = args[i+1]
				i++
			}
		default:
			if !strings.HasPrefix(args[i], "-") && name == "" {
				name = args[i]
			}
		}
	}

	resp, err := h.client.CreateSubscription(ctx, clientID, name)
	if err != nil {
		return fmt.Errorf("failed to create subscription: %w", err)
	}

	if h.formatter.Format == FormatJSON {
		h.formatter.PrintJSON(resp)
		return nil
	}
	if h.formatter.Format == FormatRaw {
		h.formatter.PrintRaw(resp)
		return nil
	}

	fmt.Fprintln(h.formatter.Out, h.formatter.color(colorGreen+colorBold, "✓ Subscription Created Successfully!"))
	fmt.Fprintf(h.formatter.Out, "  Subscription ID: %s\n", h.formatter.color(colorCyan+colorBold, resp.SubscriptionID))
	fmt.Fprintf(h.formatter.Out, "  Client ID:       %s\n", resp.ClientID)
	if resp.DisplayName != nil {
		fmt.Fprintf(h.formatter.Out, "  Display Name:    %s\n", *resp.DisplayName)
	}
	return nil
}

func (h *CommandHandler) CmdSubList(ctx context.Context, args []string) error {
	if hasHelpFlag(args) || len(args) == 0 {
		fmt.Println("Usage: i3x sub list <subscriptionId...> [--client-id <id>]")
		fmt.Println()
		fmt.Println("List active subscriptions and monitored objects (POST /v1/subscriptions/list).")
		return nil
	}
	clientID := ""
	var subIDs []string

	for i := 0; i < len(args); i++ {
		if (args[i] == "--client-id" || args[i] == "-c") && i+1 < len(args) {
			clientID = args[i+1]
			i++
		} else if !strings.HasPrefix(args[i], "-") {
			subIDs = append(subIDs, args[i])
		}
	}

	if len(subIDs) == 0 {
		return errors.New("usage: i3x sub list <subscriptionId...> [--client-id <id>]")
	}

	bulk, err := h.client.ListSubscriptions(ctx, clientID, subIDs)
	if err != nil {
		return fmt.Errorf("failed to list subscriptions: %w", err)
	}

	h.formatter.PrintSubscriptionsList(bulk)
	return nil
}

func (h *CommandHandler) CmdSubRegister(ctx context.Context, args []string) error {
	if hasHelpFlag(args) || len(args) < 2 {
		fmt.Println("Usage: i3x sub register <subscriptionId> <elementId...> [options]")
		fmt.Println()
		fmt.Println("Register objects to monitor in a subscription (POST /v1/subscriptions/register).")
		fmt.Println()
		fmt.Println("Options:")
		fmt.Println("  --depth <n>, -d <n>     Maximum composition depth")
		fmt.Println("  --client-id <id>, -c <id> Client ID")
		fmt.Println("  -h, --help              Show this help text")
		return nil
	}

	subID := args[0]
	var elementIDs []string
	clientID := ""
	var maxDepth *int

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--client-id", "-c":
			if i+1 < len(args) {
				clientID = args[i+1]
				i++
			}
		case "--depth", "-d":
			if i+1 < len(args) {
				if d, err := strconv.Atoi(args[i+1]); err == nil {
					maxDepth = &d
				}
				i++
			}
		default:
			if !strings.HasPrefix(args[i], "-") {
				elementIDs = append(elementIDs, args[i])
			}
		}
	}

	if len(elementIDs) == 0 {
		return errors.New("no element IDs to register")
	}

	bulk, err := h.client.RegisterMonitoredItems(ctx, clientID, subID, elementIDs, maxDepth)
	if err != nil {
		return fmt.Errorf("failed to register items: %w", err)
	}

	h.formatter.PrintBulkGeneric(bulk, "Monitored item registered", "Failed to register item")
	return nil
}

func (h *CommandHandler) CmdSubUnregister(ctx context.Context, args []string) error {
	if hasHelpFlag(args) || len(args) < 2 {
		fmt.Println("Usage: i3x sub unregister <subscriptionId> <elementId...> [options]")
		fmt.Println()
		fmt.Println("Unregister objects from a subscription (POST /v1/subscriptions/unregister).")
		fmt.Println()
		fmt.Println("Options:")
		fmt.Println("  --client-id <id>, -c <id> Client ID")
		fmt.Println("  -h, --help              Show this help text")
		return nil
	}

	subID := args[0]
	var elementIDs []string
	clientID := ""

	for i := 1; i < len(args); i++ {
		if (args[i] == "--client-id" || args[i] == "-c") && i+1 < len(args) {
			clientID = args[i+1]
			i++
		} else if !strings.HasPrefix(args[i], "-") {
			elementIDs = append(elementIDs, args[i])
		}
	}

	if len(elementIDs) == 0 {
		return errors.New("no element IDs to unregister")
	}

	bulk, err := h.client.UnregisterMonitoredItems(ctx, clientID, subID, elementIDs)
	if err != nil {
		return fmt.Errorf("failed to unregister items: %w", err)
	}

	h.formatter.PrintBulkGeneric(bulk, "Monitored item removed", "Failed to remove item")
	return nil
}

func (h *CommandHandler) CmdSubSync(ctx context.Context, args []string) error {
	if hasHelpFlag(args) || len(args) == 0 {
		fmt.Println("Usage: i3x sub sync <subscriptionId> [options]")
		fmt.Println()
		fmt.Println("Poll pending updates for a subscription (POST /v1/subscriptions/sync).")
		fmt.Println()
		fmt.Println("Options:")
		fmt.Println("  --ack-seq <n>, -s <n>   Last acknowledged sequence number")
		fmt.Println("  --client-id <id>, -c <id> Client ID")
		fmt.Println("  -h, --help              Show this help text")
		return nil
	}

	subID := args[0]
	clientID := ""
	var lastSeq *int

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--client-id", "-c":
			if i+1 < len(args) {
				clientID = args[i+1]
				i++
			}
		case "--ack-seq", "-s", "--last-sequence-number":
			if i+1 < len(args) {
				if s, err := strconv.Atoi(args[i+1]); err == nil {
					lastSeq = &s
				}
				i++
			}
		}
	}

	batches, err := h.client.SyncSubscription(ctx, clientID, subID, lastSeq)
	if err != nil {
		return fmt.Errorf("failed to sync subscription: %w", err)
	}

	h.formatter.PrintSyncBatches(batches)
	return nil
}

func (h *CommandHandler) CmdSubStream(ctx context.Context, args []string) error {
	if hasHelpFlag(args) || len(args) == 0 {
		fmt.Println("Usage: i3x sub stream <subscriptionId> [options]")
		fmt.Println()
		fmt.Println("Stream real-time updates via Server-Sent Events (SSE) (GET /v1/subscriptions/stream).")
		fmt.Println()
		fmt.Println("Options:")
		fmt.Println("  --client-id <id>, -c <id> Client ID")
		fmt.Println("  -h, --help              Show this help text")
		return nil
	}

	subID := args[0]
	clientID := ""

	for i := 1; i < len(args); i++ {
		if (args[i] == "--client-id" || args[i] == "-c") && i+1 < len(args) {
			clientID = args[i+1]
			i++
		}
	}

	fmt.Fprintf(h.formatter.Out, "%s (SubID: %s, ClientID: %s)... Press Ctrl+C to stop.\n",
		h.formatter.color(colorGreen+colorBold, "Connecting to SSE stream"),
		h.formatter.color(colorCyan, subID),
		clientID)

	return h.client.StreamSubscription(ctx, clientID, subID, func(event SSEEvent) error {
		h.formatter.PrintLiveStreamEvent(event)
		return nil
	})
}

func (h *CommandHandler) CmdSubDelete(ctx context.Context, args []string) error {
	if hasHelpFlag(args) || len(args) == 0 {
		fmt.Println("Usage: i3x sub delete <subscriptionId...> [options]")
		fmt.Println()
		fmt.Println("Delete active subscriptions (DELETE /v1/subscriptions/list).")
		fmt.Println()
		fmt.Println("Options:")
		fmt.Println("  --client-id <id>, -c <id> Client ID")
		fmt.Println("  -h, --help              Show this help text")
		return nil
	}

	var subIDs []string
	clientID := ""

	for i := 0; i < len(args); i++ {
		if (args[i] == "--client-id" || args[i] == "-c") && i+1 < len(args) {
			clientID = args[i+1]
			i++
		} else if !strings.HasPrefix(args[i], "-") {
			subIDs = append(subIDs, args[i])
		}
	}

	if len(subIDs) == 0 {
		return errors.New("usage: i3x sub delete <subscriptionId...> [--client-id <id>]")
	}

	bulk, err := h.client.DeleteSubscriptions(ctx, clientID, subIDs)
	if err != nil {
		return fmt.Errorf("failed to delete subscriptions: %w", err)
	}

	h.formatter.PrintBulkGeneric(bulk, "Subscription deleted", "Failed to delete subscription")
	return nil
}

// -------------------------------------------------------------
// 9. High-Level Watch / Live Monitor
// -------------------------------------------------------------

func (h *CommandHandler) CmdWatch(ctx context.Context, args []string) error {
	if hasHelpFlag(args) || len(args) == 0 {
		fmt.Println("Usage: i3x watch <elementId...> [options]")
		fmt.Println()
		fmt.Println("Live telemetry stream (automatically creates subscription, registers items, and cleans up on exit).")
		fmt.Println()
		fmt.Println("Options:")
		fmt.Println("  --depth <n>, -d <n>       Maximum composition depth")
		fmt.Println("  --name <name>, -n <name>  Subscription display name")
		fmt.Println("  --poll <interval>         Use Sync polling instead of SSE stream (e.g. --poll 1s)")
		fmt.Println("  --no-initial              Skip initial value snapshot query")
		fmt.Println("  -h, --help                Show this help text")
		return nil
	}

	var elementIDs []string
	name := "i3x-watch"
	var maxDepth *int
	pollInterval := time.Duration(0)
	showInitial := true

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--name", "-n":
			if i+1 < len(args) {
				name = args[i+1]
				i++
			}
		case "--depth", "-d":
			if i+1 < len(args) {
				if d, err := strconv.Atoi(args[i+1]); err == nil {
					maxDepth = &d
				}
				i++
			}
		case "--poll", "-p":
			pollInterval = 1 * time.Second
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				if d, err := time.ParseDuration(args[i+1]); err == nil {
					pollInterval = d
					i++
				}
			}
		case "--no-initial":
			showInitial = false
		default:
			if !strings.HasPrefix(args[i], "-") {
				elementIDs = append(elementIDs, args[i])
			}
		}
	}

	if len(elementIDs) == 0 {
		return errors.New("no element IDs specified to watch")
	}

	clientID := h.client.cfg.ClientID
	if clientID == "" {
		clientID = defaultClientID()
	}

	fmt.Fprintf(h.formatter.Out, "%s Creating subscription for %d element(s)...\n",
		h.formatter.color(colorYellow, "•"), len(elementIDs))

	subResp, err := h.client.CreateSubscription(ctx, clientID, name)
	if err != nil {
		return fmt.Errorf("failed to create watch subscription: %w", err)
	}
	subID := subResp.SubscriptionID
	if subResp.ClientID != "" {
		clientID = subResp.ClientID
	}

	// Setup cleanup on exit / interrupt
	cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cleanupCancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Fprintf(h.formatter.Out, "\n%s Cleaning up subscription %s...\n",
			h.formatter.color(colorYellow, "•"), subID)
		_, _ = h.client.DeleteSubscriptions(cleanupCtx, clientID, []string{subID})
		os.Exit(0)
	}()

	defer func() {
		_, _ = h.client.DeleteSubscriptions(cleanupCtx, clientID, []string{subID})
	}()

	// Register items
	depthVal := 1
	if maxDepth != nil {
		depthVal = *maxDepth
	}
	regBulk, err := h.client.RegisterMonitoredItems(ctx, clientID, subID, elementIDs, maxDepth)
	if err != nil {
		return fmt.Errorf("failed to register items: %w", err)
	}
	registeredCount := 0
	for _, item := range regBulk.Results {
		if !item.Success {
			id := "-"
			if item.ElementID != nil {
				id = *item.ElementID
			}
			errDetail := ""
			if item.ResponseDetail != nil {
				errDetail = ": " + item.ResponseDetail.Error()
			}
			fmt.Fprintf(h.formatter.Out, "%s Failed to register %s%s\n", h.formatter.color(colorRed, "✗"), id, errDetail)
		} else {
			registeredCount++
		}
	}

	if registeredCount == 0 && len(regBulk.Results) > 0 {
		return fmt.Errorf("failed to register any of the specified element IDs on subscription")
	}

	// Query and show initial current value snapshot
	if showInitial {
		initBulk, initErr := h.client.QueryLastKnownValues(ctx, elementIDs, depthVal)
		if initErr == nil && initBulk != nil && len(initBulk.Results) > 0 {
			hasValue := false
			for _, item := range initBulk.Results {
				if item.Success && item.Result != nil {
					hasValue = true
					id := "-"
					if item.ElementID != nil {
						id = *item.ElementID
					}
					ts := item.Result.Timestamp
					if ts == "" {
						ts = FormatTimeRFC3339(time.Now())
					}
					fmt.Fprintf(h.formatter.Out, "[%s] %s %s = %s (%s)\n",
						h.formatter.color(colorDim, ts),
						h.formatter.color(colorYellow, "[INITIAL]"),
						h.formatter.color(colorCyan+colorBold, id),
						formatValue(item.Result.Value),
						h.formatter.fmtQuality(item.Result.Quality))
				}
			}
			if !hasValue {
				fmt.Fprintln(h.formatter.Out, h.formatter.color(colorDim, "• No initial value recorded on server yet."))
			}
		}
	}

	// Polling mode fallback
	if pollInterval > 0 {
		fmt.Fprintf(h.formatter.Out, "%s %s (SubID: %s, Interval: %v)... Press Ctrl+C to exit.\n",
			h.formatter.color(colorGreen, "✓"),
			h.formatter.color(colorBold, "Polling subscription updates via sync"),
			h.formatter.color(colorCyan, subID),
			pollInterval)

		ticker := time.NewTicker(pollInterval)
		defer ticker.Stop()
		var lastSeq *int

		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-ticker.C:
				batches, syncErr := h.client.SyncSubscription(ctx, clientID, subID, lastSeq)
				if syncErr != nil {
					fmt.Fprintf(h.formatter.Out, "%s Sync error: %v\n", h.formatter.color(colorRed, "✗"), syncErr)
					continue
				}
				for _, b := range batches {
					lastSeq = &b.SequenceNumber
					for _, u := range b.Updates {
						fmt.Fprintf(h.formatter.Out, "[%s] #%d %s = %s (%s)\n",
							h.formatter.color(colorDim, u.Timestamp),
							b.SequenceNumber,
							h.formatter.color(colorCyan+colorBold, u.ElementID),
							formatValue(u.Value),
							h.formatter.fmtQuality(u.Quality))
					}
				}
			}
		}
	}

	// SSE Streaming mode
	fmt.Fprintf(h.formatter.Out, "%s %s (SubID: %s)... Press Ctrl+C to exit.\n",
		h.formatter.color(colorGreen, "✓"),
		h.formatter.color(colorBold, "Streaming real-time updates via SSE"),
		h.formatter.color(colorCyan, subID))

	err = h.client.StreamSubscription(ctx, clientID, subID, func(event SSEEvent) error {
		h.formatter.PrintLiveStreamEvent(event)
		return nil
	})
	if err != nil {
		fmt.Fprintf(h.formatter.Out, "%s Stream ended: %v\n", h.formatter.color(colorRed, "•"), err)
		return err
	}
	return nil
}

// Helper to parse relative duration (e.g. -1h, -15m, 1h ago) or RFC3339 timestamps
func parseOrFormatTimestamp(s string) string {
	s = strings.TrimSpace(s)
	if s == "now" {
		return FormatTimeRFC3339(time.Now())
	}

	// Try relative duration like -1h, -15m, 1h, 15m, -24h
	trimmed := strings.TrimPrefix(s, "-")
	trimmed = strings.TrimSuffix(trimmed, " ago")
	if d, err := time.ParseDuration(trimmed); err == nil {
		return FormatTimeRFC3339(time.Now().Add(-d))
	}

	// Try standard formats
	formats := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02",
	}
	for _, layout := range formats {
		if t, err := time.Parse(layout, s); err == nil {
			return FormatTimeRFC3339(t)
		}
	}

	return s
}
