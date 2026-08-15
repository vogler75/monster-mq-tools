package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"
)

// ANSI color codes
const (
	colorReset  = "\033[0m"
	colorBold   = "\033[1m"
	colorDim    = "\033[2m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorPurple = "\033[35m"
	colorCyan   = "\033[36m"
	colorWhite  = "\033[37m"
)

// OutputFormat defines available formatting options.
type OutputFormat string

const (
	FormatTable OutputFormat = "table"
	FormatJSON  OutputFormat = "json"
	FormatRaw   OutputFormat = "raw"
	FormatCSV   OutputFormat = "csv"
	FormatTree  OutputFormat = "tree"
)

// Formatter handles output presentation.
type Formatter struct {
	Format  OutputFormat
	NoColor bool
	Out     io.Writer
}

// NewFormatter creates a Formatter writing to os.Stdout by default.
func NewFormatter(format OutputFormat, noColor bool) *Formatter {
	if format == "" {
		format = FormatTable
	}
	return &Formatter{
		Format:  format,
		NoColor: noColor,
		Out:     os.Stdout,
	}
}

// SetFormat updates the current output format.
func (f *Formatter) SetFormat(fmtStr string) {
	f.Format = OutputFormat(strings.ToLower(fmtStr))
}

func (f *Formatter) color(code, text string) string {
	if f.NoColor {
		return text
	}
	return code + text + colorReset
}

func (f *Formatter) PrintJSON(v interface{}) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fmt.Fprintf(f.Out, "%v\n", v)
		return
	}
	fmt.Fprintln(f.Out, string(data))
}

func (f *Formatter) PrintRaw(v interface{}) {
	data, err := json.Marshal(v)
	if err != nil {
		fmt.Fprintf(f.Out, "%v\n", v)
		return
	}
	fmt.Fprintln(f.Out, string(data))
}

// PrintServerInfo formats ServerInfo
func (f *Formatter) PrintServerInfo(info *ServerInfo) {
	if f.Format == FormatJSON {
		f.PrintJSON(info)
		return
	}
	if f.Format == FormatRaw {
		f.PrintRaw(info)
		return
	}

	name := "i3X Server"
	if info.ServerName != nil && *info.ServerName != "" {
		name = *info.ServerName
	}
	ver := "unknown"
	if info.ServerVersion != nil && *info.ServerVersion != "" {
		ver = *info.ServerVersion
	}

	fmt.Fprintln(f.Out, f.color(colorBold+colorCyan, "=== i3X Server Information ==="))
	w := tabwriter.NewWriter(f.Out, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "%s\t%s\n", f.color(colorBold, "Server Name:"), name)
	fmt.Fprintf(w, "%s\t%s\n", f.color(colorBold, "Spec Version:"), info.SpecVersion)
	fmt.Fprintf(w, "%s\t%s\n", f.color(colorBold, "Server Version:"), ver)
	fmt.Fprintf(w, "%s\t\n", f.color(colorBold, "Capabilities:"))
	fmt.Fprintf(w, "  %s\t%s\n", "- Query History:", f.fmtBool(info.Capabilities.Query.History))
	fmt.Fprintf(w, "  %s\t%s\n", "- Update Current:", f.fmtBool(info.Capabilities.Update.Current))
	fmt.Fprintf(w, "  %s\t%s\n", "- Update History:", f.fmtBool(info.Capabilities.Update.History))
	fmt.Fprintf(w, "  %s\t%s\n", "- Subscribe Stream (SSE):", f.fmtBool(info.Capabilities.Subscribe.Stream))
	w.Flush()
	fmt.Fprintln(f.Out)
}

// PrintNamespaces formats Namespaces
func (f *Formatter) PrintNamespaces(namespaces []Namespace) {
	if f.Format == FormatJSON {
		f.PrintJSON(namespaces)
		return
	}
	if f.Format == FormatRaw {
		f.PrintRaw(namespaces)
		return
	}
	if f.Format == FormatCSV {
		w := csv.NewWriter(f.Out)
		w.Write([]string{"URI", "DisplayName"})
		for _, ns := range namespaces {
			w.Write([]string{ns.URI, ns.DisplayName})
		}
		w.Flush()
		return
	}

	if len(namespaces) == 0 {
		fmt.Fprintln(f.Out, f.color(colorDim, "No namespaces found."))
		return
	}

	w := tabwriter.NewWriter(f.Out, 0, 0, 3, ' ', 0)
	fmt.Fprintf(w, "%s\t%s\n", f.color(colorBold, "URI"), f.color(colorBold, "DISPLAY NAME"))
	for _, ns := range namespaces {
		fmt.Fprintf(w, "%s\t%s\n", f.color(colorCyan, ns.URI), ns.DisplayName)
	}
	w.Flush()
	fmt.Fprintf(f.Out, "\n%s %d namespace(s)\n", f.color(colorDim, "Total:"), len(namespaces))
}

// PrintObjectTypes formats ObjectTypeResponse items
func (f *Formatter) PrintObjectTypes(types []ObjectTypeResponse) {
	if f.Format == FormatJSON {
		f.PrintJSON(types)
		return
	}
	if f.Format == FormatRaw {
		f.PrintRaw(types)
		return
	}
	if f.Format == FormatCSV {
		w := csv.NewWriter(f.Out)
		w.Write([]string{"ElementID", "DisplayName", "NamespaceURI", "SourceTypeID", "Version"})
		for _, t := range types {
			ver := ""
			if t.Version != nil {
				ver = *t.Version
			}
			w.Write([]string{t.ElementID, t.DisplayName, t.NamespaceURI, t.SourceTypeID, ver})
		}
		w.Flush()
		return
	}

	if len(types) == 0 {
		fmt.Fprintln(f.Out, f.color(colorDim, "No object types found."))
		return
	}

	w := tabwriter.NewWriter(f.Out, 0, 0, 3, ' ', 0)
	fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
		f.color(colorBold, "ELEMENT ID"),
		f.color(colorBold, "DISPLAY NAME"),
		f.color(colorBold, "SOURCE TYPE ID"),
		f.color(colorBold, "VERSION"),
		f.color(colorBold, "NAMESPACE URI"))

	for _, t := range types {
		ver := "-"
		if t.Version != nil && *t.Version != "" {
			ver = *t.Version
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			f.color(colorCyan, t.ElementID),
			t.DisplayName,
			t.SourceTypeID,
			ver,
			f.color(colorDim, t.NamespaceURI))
	}
	w.Flush()
	fmt.Fprintf(f.Out, "\n%s %d object type(s)\n", f.color(colorDim, "Total:"), len(types))
}

// PrintRelationshipTypes formats RelationshipType items
func (f *Formatter) PrintRelationshipTypes(relTypes []RelationshipType) {
	if f.Format == FormatJSON {
		f.PrintJSON(relTypes)
		return
	}
	if f.Format == FormatRaw {
		f.PrintRaw(relTypes)
		return
	}
	if f.Format == FormatCSV {
		w := csv.NewWriter(f.Out)
		w.Write([]string{"ElementID", "DisplayName", "RelationshipID", "ReverseOf", "NamespaceURI"})
		for _, r := range relTypes {
			w.Write([]string{r.ElementID, r.DisplayName, r.RelationshipID, r.ReverseOf, r.NamespaceURI})
		}
		w.Flush()
		return
	}

	if len(relTypes) == 0 {
		fmt.Fprintln(f.Out, f.color(colorDim, "No relationship types found."))
		return
	}

	w := tabwriter.NewWriter(f.Out, 0, 0, 3, ' ', 0)
	fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
		f.color(colorBold, "ELEMENT ID"),
		f.color(colorBold, "DISPLAY NAME"),
		f.color(colorBold, "RELATIONSHIP ID"),
		f.color(colorBold, "REVERSE OF"),
		f.color(colorBold, "NAMESPACE URI"))

	for _, r := range relTypes {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			f.color(colorCyan, r.ElementID),
			r.DisplayName,
			r.RelationshipID,
			r.ReverseOf,
			f.color(colorDim, r.NamespaceURI))
	}
	w.Flush()
	fmt.Fprintf(f.Out, "\n%s %d relationship type(s)\n", f.color(colorDim, "Total:"), len(relTypes))
}

// PrintObjects formats ObjectInstanceResponse items
func (f *Formatter) PrintObjects(objects []ObjectInstanceResponse) {
	if f.Format == FormatJSON {
		f.PrintJSON(objects)
		return
	}
	if f.Format == FormatRaw {
		f.PrintRaw(objects)
		return
	}
	if f.Format == FormatCSV {
		w := csv.NewWriter(f.Out)
		w.Write([]string{"ElementID", "DisplayName", "TypeElementID", "ParentID", "IsComposition", "IsExtended"})
		for _, o := range objects {
			parent := ""
			if o.ParentID != nil {
				parent = *o.ParentID
			}
			w.Write([]string{o.ElementID, o.DisplayName, o.TypeElementID, parent, fmt.Sprintf("%v", o.IsComposition), fmt.Sprintf("%v", o.IsExtended)})
		}
		w.Flush()
		return
	}
	if f.Format == FormatTree {
		f.PrintObjectsTree(objects)
		return
	}

	if len(objects) == 0 {
		fmt.Fprintln(f.Out, f.color(colorDim, "No objects found."))
		return
	}

	w := tabwriter.NewWriter(f.Out, 0, 0, 3, ' ', 0)
	fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
		f.color(colorBold, "ELEMENT ID"),
		f.color(colorBold, "DISPLAY NAME"),
		f.color(colorBold, "TYPE"),
		f.color(colorBold, "PARENT"),
		f.color(colorBold, "COMPOSITION"))

	for _, o := range objects {
		parent := "-"
		if o.ParentID != nil && *o.ParentID != "" {
			parent = *o.ParentID
		}
		comp := "false"
		if o.IsComposition {
			comp = f.color(colorYellow, "true")
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			f.color(colorCyan, o.ElementID),
			o.DisplayName,
			f.color(colorPurple, o.TypeElementID),
			parent,
			comp)
	}
	w.Flush()
	fmt.Fprintf(f.Out, "\n%s %d object(s)\n", f.color(colorDim, "Total:"), len(objects))
}

// PrintObjectsTree prints hierarchical object tree
func (f *Formatter) PrintObjectsTree(objects []ObjectInstanceResponse) {
	childrenMap := make(map[string][]ObjectInstanceResponse)
	var roots []ObjectInstanceResponse

	for _, o := range objects {
		if o.ParentID == nil || *o.ParentID == "" {
			roots = append(roots, o)
		} else {
			childrenMap[*o.ParentID] = append(childrenMap[*o.ParentID], o)
		}
	}

	var printNode func(o ObjectInstanceResponse, prefix string, isLast bool)
	printNode = func(o ObjectInstanceResponse, prefix string, isLast bool) {
		marker := "├── "
		newPrefix := prefix + "│   "
		if isLast {
			marker = "└── "
			newPrefix = prefix + "    "
		}
		fmt.Fprintf(f.Out, "%s%s%s %s [%s]\n",
			prefix,
			marker,
			f.color(colorCyan+colorBold, o.ElementID),
			f.color(colorDim, "("+o.DisplayName+")"),
			f.color(colorPurple, o.TypeElementID))

		children := childrenMap[o.ElementID]
		for i, child := range children {
			printNode(child, newPrefix, i == len(children)-1)
		}
	}

	if len(roots) == 0 && len(objects) > 0 {
		roots = objects
	}

	for i, r := range roots {
		printNode(r, "", i == len(roots)-1)
	}
}

// PrintCurrentValues formats CurrentValue results
func (f *Formatter) PrintCurrentValues(bulk *BulkResponse[CurrentValueResult]) {
	if f.Format == FormatJSON {
		f.PrintJSON(bulk)
		return
	}
	if f.Format == FormatRaw {
		f.PrintRaw(bulk)
		return
	}

	w := tabwriter.NewWriter(f.Out, 0, 0, 3, ' ', 0)
	fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
		f.color(colorBold, "ELEMENT ID"),
		f.color(colorBold, "VALUE"),
		f.color(colorBold, "QUALITY"),
		f.color(colorBold, "TIMESTAMP"),
		f.color(colorBold, "STATUS"))

	for _, item := range bulk.Results {
		id := "-"
		if item.ElementID != nil {
			id = *item.ElementID
		}
		if !item.Success || item.Result == nil {
			errStr := "Failed"
			if item.ResponseDetail != nil {
				errStr = item.ResponseDetail.Error()
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
				f.color(colorCyan, id),
				"-",
				"-",
				"-",
				f.color(colorRed, errStr))
			continue
		}

		res := item.Result
		valStr := formatValue(res.Value)
		qualStr := f.fmtQuality(res.Quality)
		tsStr := f.color(colorDim, res.Timestamp)
		status := f.color(colorGreen, "OK")

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			f.color(colorCyan, id),
			valStr,
			qualStr,
			tsStr,
			status)

		// If composition components exist
		if len(res.Components) > 0 {
			var compKeys []string
			for k := range res.Components {
				compKeys = append(compKeys, k)
			}
			sort.Strings(compKeys)
			for _, k := range compKeys {
				cv := res.Components[k]
				fmt.Fprintf(w, "  %s\t%s\t%s\t%s\t%s\n",
					f.color(colorDim, "↳ "+k),
					formatValue(cv.Value),
					f.fmtQuality(cv.Quality),
					f.color(colorDim, cv.Timestamp),
					"")
			}
		}
	}
	w.Flush()
}

// PrintHistoricalValues formats HistoricalValue results
func (f *Formatter) PrintHistoricalValues(bulk *BulkResponse[HistoricalValueResult]) {
	if f.Format == FormatJSON {
		f.PrintJSON(bulk)
		return
	}
	if f.Format == FormatRaw {
		f.PrintRaw(bulk)
		return
	}

	for _, item := range bulk.Results {
		id := "-"
		if item.ElementID != nil {
			id = *item.ElementID
		}
		if !item.Success || item.Result == nil {
			errStr := "Failed"
			if item.ResponseDetail != nil {
				errStr = item.ResponseDetail.Error()
			}
			fmt.Fprintf(f.Out, "%s: %s\n", f.color(colorCyan+colorBold, id), f.color(colorRed, errStr))
			continue
		}

		res := item.Result
		fmt.Fprintf(f.Out, "%s: %d record(s)\n", f.color(colorCyan+colorBold, id), len(res.Values))

		w := tabwriter.NewWriter(f.Out, 0, 0, 3, ' ', 0)
		fmt.Fprintf(w, "  %s\t%s\t%s\n",
			f.color(colorBold, "TIMESTAMP"),
			f.color(colorBold, "VALUE"),
			f.color(colorBold, "QUALITY"))

		for _, v := range res.Values {
			fmt.Fprintf(w, "  %s\t%s\t%s\n",
				f.color(colorDim, v.Timestamp),
				formatValue(v.Value),
				f.fmtQuality(v.Quality))
		}
		w.Flush()
		fmt.Fprintln(f.Out)
	}
}

// PrintSubscriptionsList formats ListSubscriptions results
func (f *Formatter) PrintSubscriptionsList(bulk *BulkResponse[SubscriptionDetail]) {
	if f.Format == FormatJSON {
		f.PrintJSON(bulk)
		return
	}
	if f.Format == FormatRaw {
		f.PrintRaw(bulk)
		return
	}

	w := tabwriter.NewWriter(f.Out, 0, 0, 3, ' ', 0)
	fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
		f.color(colorBold, "SUBSCRIPTION ID"),
		f.color(colorBold, "DISPLAY NAME"),
		f.color(colorBold, "MONITORED ITEMS"),
		f.color(colorBold, "STATUS"))

	for _, item := range bulk.Results {
		subID := "-"
		if item.SubscriptionID != nil {
			subID = *item.SubscriptionID
		}
		if !item.Success || item.Result == nil {
			errStr := "Failed"
			if item.ResponseDetail != nil {
				errStr = item.ResponseDetail.Error()
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
				f.color(colorCyan, subID),
				"-",
				"-",
				f.color(colorRed, errStr))
			continue
		}

		res := item.Result
		name := "-"
		if res.DisplayName != nil && *res.DisplayName != "" {
			name = *res.DisplayName
		}
		var items []string
		for _, mo := range res.MonitoredObjects {
			depth := ""
			if mo.MaxDepth != nil {
				depth = fmt.Sprintf(" (depth=%d)", *mo.MaxDepth)
			}
			items = append(items, mo.ElementID+depth)
		}
		itemList := strings.Join(items, ", ")
		if itemList == "" {
			itemList = "(none)"
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
			f.color(colorCyan, res.SubscriptionID),
			name,
			itemList,
			f.color(colorGreen, "Active"))
	}
	w.Flush()
}

// PrintSyncBatches formats SyncBatches
func (f *Formatter) PrintSyncBatches(batches []SyncBatch) {
	if f.Format == FormatJSON {
		f.PrintJSON(batches)
		return
	}
	if f.Format == FormatRaw {
		f.PrintRaw(batches)
		return
	}

	if len(batches) == 0 {
		fmt.Fprintln(f.Out, f.color(colorDim, "No pending batches returned (all up-to-date)."))
		return
	}

	totalUpdates := 0
	for _, b := range batches {
		totalUpdates += len(b.Updates)
	}

	fmt.Fprintf(f.Out, "Received %d batch(es) with %d update(s):\n", len(batches), totalUpdates)
	w := tabwriter.NewWriter(f.Out, 0, 0, 3, ' ', 0)
	fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
		f.color(colorBold, "SEQ"),
		f.color(colorBold, "ELEMENT ID"),
		f.color(colorBold, "VALUE"),
		f.color(colorBold, "QUALITY"),
		f.color(colorBold, "TIMESTAMP"))

	for _, b := range batches {
		for _, u := range b.Updates {
			fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\n",
				b.SequenceNumber,
				f.color(colorCyan, u.ElementID),
				formatValue(u.Value),
				f.fmtQuality(u.Quality),
				f.color(colorDim, u.Timestamp))
		}
	}
	w.Flush()
}

// PrintBulkGeneric formats generic bulk operations (such as register, unregister, write, delete)
func (f *Formatter) PrintBulkGeneric(bulk *BulkResponse[interface{}], successMsg, failMsg string) {
	if f.Format == FormatJSON {
		f.PrintJSON(bulk)
		return
	}
	if f.Format == FormatRaw {
		f.PrintRaw(bulk)
		return
	}

	w := tabwriter.NewWriter(f.Out, 0, 0, 3, ' ', 0)
	fmt.Fprintf(w, "%s\t%s\t%s\n",
		f.color(colorBold, "ELEMENT/SUBSCRIPTION ID"),
		f.color(colorBold, "STATUS"),
		f.color(colorBold, "DETAIL"))

	for _, item := range bulk.Results {
		id := "-"
		if item.ElementID != nil {
			id = *item.ElementID
		} else if item.SubscriptionID != nil {
			id = *item.SubscriptionID
		}

		if item.Success {
			fmt.Fprintf(w, "%s\t%s\t%s\n",
				f.color(colorCyan, id),
				f.color(colorGreen, "Success"),
				successMsg)
		} else {
			detail := failMsg
			if item.ResponseDetail != nil {
				detail = item.ResponseDetail.Error()
			}
			fmt.Fprintf(w, "%s\t%s\t%s\n",
				f.color(colorCyan, id),
				f.color(colorRed, "Failed"),
				detail)
		}
	}
	w.Flush()
}

// PrintLiveStreamEvent prints a live SSE event to the terminal
func (f *Formatter) PrintLiveStreamEvent(event SSEEvent) {
	if strings.TrimSpace(event.Data) == "" {
		return
	}

	if f.Format == FormatJSON || f.Format == FormatRaw {
		fmt.Fprintf(f.Out, "%s\r\n", event.Data)
		return
	}

	dataBytes := []byte(event.Data)

	// 1. Try flat array of SyncUpdateEntry: [{"elementId": "...", "value": ...}, ...]
	var entries []SyncUpdateEntry
	if err := json.Unmarshal(dataBytes, &entries); err == nil && len(entries) > 0 && entries[0].ElementID != "" {
		for _, entry := range entries {
			ts := entry.Timestamp
			if ts == "" {
				ts = FormatTimeRFC3339(time.Now())
			}
			fmt.Fprintf(f.Out, "[%s] %s = %s (%s)\r\n",
				f.color(colorDim, ts),
				f.color(colorCyan+colorBold, entry.ElementID),
				formatValue(entry.Value),
				f.fmtQuality(entry.Quality))
		}
		return
	}

	// 2. Try array of SyncBatch: [{"sequenceNumber": 1, "updates": [...]}]
	var batches []SyncBatch
	if err := json.Unmarshal(dataBytes, &batches); err == nil && len(batches) > 0 && len(batches[0].Updates) > 0 {
		for _, b := range batches {
			for _, u := range b.Updates {
				ts := u.Timestamp
				if ts == "" {
					ts = FormatTimeRFC3339(time.Now())
				}
				fmt.Fprintf(f.Out, "[%s] #%d %s = %s (%s)\r\n",
					f.color(colorDim, ts),
					b.SequenceNumber,
					f.color(colorCyan+colorBold, u.ElementID),
					formatValue(u.Value),
					f.fmtQuality(u.Quality))
			}
		}
		return
	}

	// 3. Try single SyncBatch: {"sequenceNumber": 1, "updates": [...]}
	var batch SyncBatch
	if err := json.Unmarshal(dataBytes, &batch); err == nil && (len(batch.Updates) > 0 || batch.SequenceNumber > 0) {
		for _, u := range batch.Updates {
			ts := u.Timestamp
			if ts == "" {
				ts = FormatTimeRFC3339(time.Now())
			}
			fmt.Fprintf(f.Out, "[%s] #%d %s = %s (%s)\r\n",
				f.color(colorDim, ts),
				batch.SequenceNumber,
				f.color(colorCyan+colorBold, u.ElementID),
				formatValue(u.Value),
				f.fmtQuality(u.Quality))
		}
		return
	}

	// 4. Try single SyncUpdateEntry: {"elementId": "...", "value": ...}
	var entry SyncUpdateEntry
	if err := json.Unmarshal(dataBytes, &entry); err == nil && entry.ElementID != "" {
		ts := entry.Timestamp
		if ts == "" {
			ts = FormatTimeRFC3339(time.Now())
		}
		fmt.Fprintf(f.Out, "[%s] %s = %s (%s)\r\n",
			f.color(colorDim, ts),
			f.color(colorCyan+colorBold, entry.ElementID),
			formatValue(entry.Value),
			f.fmtQuality(entry.Quality))
		return
	}

	// 5. Try wrapper objects like {"updates": [...]} or {"result": [...]}
	var wrapper struct {
		Updates []SyncUpdateEntry `json:"updates"`
		Result  any               `json:"result"`
		Results []any             `json:"results"`
	}
	if err := json.Unmarshal(dataBytes, &wrapper); err == nil && len(wrapper.Updates) > 0 {
		for _, u := range wrapper.Updates {
			ts := u.Timestamp
			if ts == "" {
				ts = FormatTimeRFC3339(time.Now())
			}
			fmt.Fprintf(f.Out, "[%s] %s = %s (%s)\r\n",
				f.color(colorDim, ts),
				f.color(colorCyan+colorBold, u.ElementID),
				formatValue(u.Value),
				f.fmtQuality(u.Quality))
		}
		return
	}

	// Fallback raw display
	label := event.Event
	if label == "" {
		label = "data"
	}
	fmt.Fprintf(f.Out, "[SSE %s] %s\r\n", f.color(colorYellow, label), event.Data)
}

func (f *Formatter) fmtBool(b bool) string {
	if b {
		return f.color(colorGreen, "true")
	}
	return f.color(colorDim, "false")
}

func (f *Formatter) fmtQuality(q string) string {
	if q == "" {
		return f.color(colorDim, "-")
	}
	lower := strings.ToLower(q)
	if strings.Contains(lower, "good") {
		return f.color(colorGreen, q)
	}
	if strings.Contains(lower, "bad") {
		return f.color(colorRed, q)
	}
	if strings.Contains(lower, "uncertain") {
		return f.color(colorYellow, q)
	}
	return q
}

func formatValue(v interface{}) string {
	if v == nil {
		return "<null>"
	}
	switch val := v.(type) {
	case string:
		return fmt.Sprintf("%q", val)
	case float64:
		return fmt.Sprintf("%v", val)
	case bool:
		return fmt.Sprintf("%v", val)
	default:
		b, _ := json.Marshal(val)
		return string(b)
	}
}
