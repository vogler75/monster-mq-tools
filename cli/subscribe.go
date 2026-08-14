package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	"golang.org/x/term"
)

const (
	graphqlTransportWSProtocol = "graphql-transport-ws"
)

// wsMessage represents a graphql-transport-ws protocol message.
type wsMessage struct {
	ID      string          `json:"id,omitempty"`
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// TopicUpdateData represents incoming topicUpdate payload.
type TopicUpdateData struct {
	Topic     string `json:"topic"`
	Payload   string `json:"payload"`
	Format    string `json:"format"`
	Timestamp int64  `json:"timestamp"`
	QoS       int    `json:"qos"`
	Retained  bool   `json:"retained"`
}

// toWebSocketURL derives the ws:// or wss:// URL from the broker GraphQL endpoint.
func toWebSocketURL(httpURL string, customWS string) string {
	if customWS != "" {
		return customWS
	}
	u, err := url.Parse(httpURL)
	if err != nil {
		if strings.HasPrefix(httpURL, "https://") {
			return "wss://" + strings.TrimPrefix(httpURL, "https://")
		}
		return "ws://" + strings.TrimPrefix(httpURL, "http://")
	}
	if strings.EqualFold(u.Scheme, "https") {
		u.Scheme = "wss"
	} else {
		u.Scheme = "ws"
	}
	return u.String()
}

// connectGraphQLWebSocket establishes a graphql-transport-ws WebSocket connection.
func connectGraphQLWebSocket(ctx context.Context, client *Client, wsURL string) (*websocket.Conn, error) {
	headers := make(http.Header)
	if client.cfg.Token != "" {
		headers.Set("Authorization", "Bearer "+client.cfg.Token)
	} else if client.cfg.Username != "" && client.cfg.Password != "" {
		headers.Set("Authorization", basicAuthHeader(client.cfg.Username, client.cfg.Password))
	}

	dialer := websocket.Dialer{
		Subprotocols:    []string{graphqlTransportWSProtocol},
		ReadBufferSize:  65536,
		WriteBufferSize: 65536,
		HandshakeTimeout: 10 * time.Second,
	}

	conn, resp, err := dialer.DialContext(ctx, wsURL, headers)
	if err != nil {
		if resp != nil {
			return nil, fmt.Errorf("WebSocket handshake failed (HTTP %d): %w", resp.StatusCode, err)
		}
		return nil, fmt.Errorf("failed to connect to WebSocket at '%s': %w", wsURL, err)
	}

	// Send connection_init
	initPayload := map[string]any{}
	if client.cfg.Token != "" {
		initPayload["authorization"] = "Bearer " + client.cfg.Token
	}
	if client.cfg.Username != "" {
		initPayload["username"] = client.cfg.Username
		initPayload["password"] = client.cfg.Password
	}

	initPayloadBytes, _ := json.Marshal(initPayload)
	initMsg := wsMessage{
		Type:    "connection_init",
		Payload: initPayloadBytes,
	}

	if err := conn.WriteJSON(initMsg); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("failed to send connection_init: %w", err)
	}

	// Wait for connection_ack
	_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	for {
		var ackMsg wsMessage
		if err := conn.ReadJSON(&ackMsg); err != nil {
			_ = conn.Close()
			return nil, fmt.Errorf("failed to receive connection_ack: %w", err)
		}

		if ackMsg.Type == "connection_ack" {
			break
		} else if ackMsg.Type == "ping" {
			_ = conn.WriteJSON(wsMessage{Type: "pong"})
		} else if ackMsg.Type == "connection_error" || ackMsg.Type == "error" {
			_ = conn.Close()
			return nil, fmt.Errorf("server rejected connection: %s", string(ackMsg.Payload))
		}
	}
	_ = conn.SetReadDeadline(time.Time{})

	return conn, nil
}

// subscribeTopicUpdates starts a topicUpdates GraphQL subscription over WebSocket.
func subscribeTopicUpdates(conn *websocket.Conn, subID string, topicFilters []string, format string) error {
	if format == "" {
		format = "JSON"
	}
	query := `
		subscription TopicUpdates($topicFilters: [String!]!, $format: DataFormat) {
			topicUpdates(topicFilters: $topicFilters, format: $format) {
				topic
				payload
				format
				timestamp
				qos
				retained
			}
		}
	`
	vars := map[string]any{
		"topicFilters": topicFilters,
		"format":       format,
	}

	subPayload := map[string]any{
		"query":     query,
		"variables": vars,
	}
	payloadBytes, _ := json.Marshal(subPayload)

	msg := wsMessage{
		ID:      subID,
		Type:    "subscribe",
		Payload: payloadBytes,
	}

	return conn.WriteJSON(msg)
}

// runSubscribe handles the `mmq subscribe <topics...>` command.
func runSubscribe(ctx context.Context, client *Client, args []string) error {
	if len(args) < 1 || hasHelpFlag(args) {
		fmt.Println("Usage: mmq subscribe <topic1> [topic2...] [options]")
		fmt.Println()
		fmt.Println("Subscribe to real-time topic updates via WebSocket.")
		fmt.Println()
		fmt.Println("Arguments:")
		fmt.Println("  <topic...>               One or more topic filters (e.g. 'sensors/#', 'factory/line1/temp')")
		fmt.Println()
		fmt.Println("Options:")
		fmt.Println("  --monitor, -m            Launch interactive full-screen live monitor table")
		fmt.Println("  --ws-url <url>           Custom WebSocket endpoint (default: derived from broker URL)")
		fmt.Println("  --format <format>        Requested payload format: JSON, TEXT, BINARY (default: JSON)")
		fmt.Println("  --raw                    Do not decode base64 string payloads")
		fmt.Println("  -h, --help               Show this help text")
		return nil
	}

	var topicFilters []string
	customWS := ""
	format := "JSON"
	monitorMode := false
	rawPayload := false

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case (arg == "--monitor" || arg == "-m"):
			monitorMode = true
		case arg == "--raw":
			rawPayload = true
		case (arg == "--ws-url" || arg == "-w") && i+1 < len(args):
			customWS = args[i+1]
			i++
		case (arg == "--format" || arg == "-f") && i+1 < len(args):
			format = strings.ToUpper(args[i+1])
			i++
		case !strings.HasPrefix(arg, "-"):
			topicFilters = append(topicFilters, arg)
		}
	}

	if len(topicFilters) == 0 {
		return fmt.Errorf("at least one topic filter is required (e.g. 'mmq subscribe sensors/#')")
	}

	if monitorMode {
		return runMonitor(ctx, client, args)
	}

	wsURL := toWebSocketURL(client.cfg.URL, customWS)
	return runSubscribeStream(ctx, client, wsURL, topicFilters, format, rawPayload)
}

// runSubscribeStream streams live topic updates to stdout.
func runSubscribeStream(ctx context.Context, client *Client, wsURL string, topicFilters []string, format string, rawPayload bool) error {
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if !client.cfg.JSONMode {
		fmt.Printf("Connecting WebSocket to %s ...\n", wsURL)
	}

	conn, err := connectGraphQLWebSocket(ctx, client, wsURL)
	if err != nil {
		return err
	}
	defer func() {
		_ = conn.WriteJSON(wsMessage{ID: "1", Type: "complete"})
		_ = conn.Close()
	}()

	if err := subscribeTopicUpdates(conn, "1", topicFilters, format); err != nil {
		return fmt.Errorf("failed to start subscription: %w", err)
	}

	if !client.cfg.JSONMode {
		fmt.Printf("✓ Subscribed to %d filter(s): %s\n", len(topicFilters), strings.Join(topicFilters, ", "))
		fmt.Println("Waiting for incoming messages... (Press Ctrl+C to stop)")
		fmt.Println(strings.Repeat("-", 75))
	}

	msgChan := make(chan TopicUpdateData, 100)
	errChan := make(chan error, 1)

	go func() {
		for {
			var incoming wsMessage
			if err := conn.ReadJSON(&incoming); err != nil {
				select {
				case <-ctx.Done():
					return
				default:
					errChan <- err
					return
				}
			}

			switch incoming.Type {
			case "next":
				var dataWrapper struct {
					Data struct {
						TopicUpdates TopicUpdateData `json:"topicUpdates"`
					} `json:"data"`
					Errors []struct {
						Message string `json:"message"`
					} `json:"errors"`
				}
				if err := json.Unmarshal(incoming.Payload, &dataWrapper); err == nil {
					if len(dataWrapper.Errors) > 0 {
						errChan <- fmt.Errorf("GraphQL subscription error: %s", dataWrapper.Errors[0].Message)
						return
					}
					msgChan <- dataWrapper.Data.TopicUpdates
				}
			case "ping":
				_ = conn.WriteJSON(wsMessage{Type: "pong"})
			case "error":
				errChan <- fmt.Errorf("subscription error: %s", string(incoming.Payload))
				return
			case "complete":
				errChan <- fmt.Errorf("subscription completed by server")
				return
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			if !client.cfg.JSONMode {
				fmt.Println("\nSubscription stopped.")
			}
			return nil
		case err := <-errChan:
			select {
			case <-ctx.Done():
				return nil
			default:
				return err
			}
		case update := <-msgChan:
			displayPayload := update.Payload
			if !rawPayload {
				displayPayload = formatPayload(update.Payload)
			}
			isoTime := formatTimestampISO(update.Timestamp)

			if client.cfg.JSONMode {
				outObj := map[string]any{
					"topic":     update.Topic,
					"payload":   displayPayload,
					"format":    update.Format,
					"timestamp": update.Timestamp,
					"isoTime":   isoTime,
					"qos":       update.QoS,
					"retained":  update.Retained,
				}
				_ = printJSON(outObj)
			} else {
				retStr := ""
				if update.Retained {
					retStr = " [RETAINED]"
				}
				fmt.Printf("[%s] Topic: %s (QoS %d)%s\n", isoTime, update.Topic, update.QoS, retStr)
				fmt.Printf("  Payload: %s\n\n", displayPayload)
			}
		}
	}
}

// TopicState tracks the latest received value and metrics for a topic in monitor mode.
type TopicState struct {
	Topic       string
	Payload     string
	Format      string
	Timestamp   int64
	QoS         int
	Retained    bool
	UpdateCount int
	LastUpdated time.Time
}

// runMonitor provides an interactive full-screen text-based live monitor table.
func runMonitor(ctx context.Context, client *Client, args []string) error {
	var topicFilters []string
	customWS := ""
	format := "JSON"
	rawPayload := false

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--raw":
			rawPayload = true
		case (arg == "--ws-url" || arg == "-w") && i+1 < len(args):
			customWS = args[i+1]
			i++
		case (arg == "--format" || arg == "-f") && i+1 < len(args):
			format = strings.ToUpper(args[i+1])
			i++
		case arg == "--monitor" || arg == "-m":
			// flag consumed
		case !strings.HasPrefix(arg, "-"):
			topicFilters = append(topicFilters, arg)
		}
	}

	if len(topicFilters) == 0 {
		topicFilters = []string{"#"}
	}

	wsURL := toWebSocketURL(client.cfg.URL, customWS)

	// Check if stdout is an interactive terminal
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		// Fallback to streaming mode if stdin is not a TTY
		return runSubscribeStream(ctx, client, wsURL, topicFilters, format, rawPayload)
	}

	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer cancel()

	fmt.Printf("Connecting monitor to %s ...\n", wsURL)
	conn, err := connectGraphQLWebSocket(ctx, client, wsURL)
	if err != nil {
		return err
	}
	defer func() {
		_ = conn.WriteJSON(wsMessage{ID: "1", Type: "complete"})
		_ = conn.Close()
	}()

	if err := subscribeTopicUpdates(conn, "1", topicFilters, format); err != nil {
		return fmt.Errorf("failed to start subscription: %w", err)
	}

	// Switch terminal to raw mode for scrolling & keypress detection
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return runSubscribeStream(ctx, client, wsURL, topicFilters, format, rawPayload)
	}
	defer func() {
		_ = term.Restore(fd, oldState)
		// Ensure cursor is visible and styles reset on exit
		fmt.Print("\033[?25h\033[0m\r\n\r\nMonitor stopped.\r\n")
	}()

	var (
		mu           sync.Mutex
		topicMap     = make(map[string]*TopicState)
		topicList    []string
		scrollOffset = 0
		sortBy       = "name" // "name", "time", "count"
		paused       = false
		redrawChan   = make(chan struct{}, 10)
		errChan      = make(chan error, 1)
	)

	triggerRedraw := func() {
		select {
		case redrawChan <- struct{}{}:
		default:
		}
	}

	// WebSocket Read Loop
	go func() {
		for {
			var incoming wsMessage
			if err := conn.ReadJSON(&incoming); err != nil {
				select {
				case <-ctx.Done():
					return
				default:
					errChan <- err
					return
				}
			}

			switch incoming.Type {
			case "next":
				var dataWrapper struct {
					Data struct {
						TopicUpdates TopicUpdateData `json:"topicUpdates"`
					} `json:"data"`
				}
				if err := json.Unmarshal(incoming.Payload, &dataWrapper); err == nil {
					up := dataWrapper.Data.TopicUpdates
					dispPayload := up.Payload
					if !rawPayload {
						dispPayload = formatPayload(up.Payload)
					}

					mu.Lock()
					if !paused {
						st, exists := topicMap[up.Topic]
						if !exists {
							st = &TopicState{Topic: up.Topic}
							topicMap[up.Topic] = st
							topicList = append(topicList, up.Topic)
						}
						st.Payload = dispPayload
						st.Format = up.Format
						st.Timestamp = up.Timestamp
						st.QoS = up.QoS
						st.Retained = up.Retained
						st.UpdateCount++
						st.LastUpdated = time.Now()
					}
					mu.Unlock()

					triggerRedraw()
				}
			case "ping":
				_ = conn.WriteJSON(wsMessage{Type: "pong"})
			case "error":
				errChan <- fmt.Errorf("subscription error: %s", string(incoming.Payload))
				return
			case "complete":
				errChan <- fmt.Errorf("subscription completed")
				return
			}
		}
	}()

	// Keyboard Input Reader
	go func() {
		buf := make([]byte, 32)
		for {
			n, err := os.Stdin.Read(buf)
			if err != nil {
				return
			}
			if n == 0 {
				continue
			}

			b := buf[:n]

			// Ctrl+C (\x03) or 'q' or 'Q'
			if b[0] == 3 || b[0] == 'q' || b[0] == 'Q' {
				cancel()
				return
			}

			mu.Lock()
			if b[0] == 'c' || b[0] == 'C' {
				// Clear topics
				topicMap = make(map[string]*TopicState)
				topicList = nil
				scrollOffset = 0
			} else if b[0] == 'p' || b[0] == 'P' {
				// Toggle pause
				paused = !paused
			} else if b[0] == 's' || b[0] == 'S' {
				// Cycle sort
				switch sortBy {
				case "name":
					sortBy = "time"
				case "time":
					sortBy = "count"
				default:
					sortBy = "name"
				}
			} else if b[0] == 'g' {
				// Jump to top
				scrollOffset = 0
			} else if b[0] == 'G' {
				// Jump to bottom
				scrollOffset = 999999
			} else if b[0] == 'k' {
				// Up
				if scrollOffset > 0 {
					scrollOffset--
				}
			} else if b[0] == 'j' {
				// Down
				scrollOffset++
			} else if b[0] == 27 { // Escape sequence
				if n >= 3 && b[1] == '[' {
					switch b[2] {
					case 'A': // Up arrow
						if scrollOffset > 0 {
							scrollOffset--
						}
					case 'B': // Down arrow
						scrollOffset++
					case '5': // Page Up
						scrollOffset -= 10
						if scrollOffset < 0 {
							scrollOffset = 0
						}
					case '6': // Page Down
						scrollOffset += 10
					case 'H', '1', '7': // Home
						scrollOffset = 0
					case 'F', '4', '8': // End
						scrollOffset = 999999
					}
				} else if n == 1 {
					// Standalone ESC -> Quit
					cancel()
					mu.Unlock()
					return
				}
			}
			mu.Unlock()

			triggerRedraw()
		}
	}()

	// Render loop
	renderTicker := time.NewTicker(50 * time.Millisecond)
	defer renderTicker.Stop()

	// Initial redraw
	triggerRedraw()

	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-errChan:
			return err
		case <-redrawChan:
			renderMonitorView(wsURL, topicFilters, &mu, topicMap, topicList, &scrollOffset, sortBy, paused)
		case <-renderTicker.C:
			renderMonitorView(wsURL, topicFilters, &mu, topicMap, topicList, &scrollOffset, sortBy, paused)
		}
	}
}

// renderMonitorView draws the live TUI dashboard table.
func renderMonitorView(wsURL string, topicFilters []string, mu *sync.Mutex, topicMap map[string]*TopicState, topicList []string, scrollOffset *int, sortBy string, paused bool) {
	mu.Lock()
	defer mu.Unlock()

	width, height, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || width < 40 || height < 10 {
		width = 100
		height = 24
	}

	// Sort topics
	sortedKeys := make([]string, len(topicList))
	copy(sortedKeys, topicList)

	switch sortBy {
	case "name":
		sort.Strings(sortedKeys)
	case "time":
		sort.Slice(sortedKeys, func(i, j int) bool {
			return topicMap[sortedKeys[i]].Timestamp > topicMap[sortedKeys[j]].Timestamp
		})
	case "count":
		sort.Slice(sortedKeys, func(i, j int) bool {
			return topicMap[sortedKeys[i]].UpdateCount > topicMap[sortedKeys[j]].UpdateCount
		})
	}

	headerLines := 5
	footerLines := 2
	visibleRows := height - headerLines - footerLines
	if visibleRows < 1 {
		visibleRows = 1
	}

	maxScroll := len(sortedKeys) - visibleRows
	if maxScroll < 0 {
		maxScroll = 0
	}
	if *scrollOffset > maxScroll {
		*scrollOffset = maxScroll
	}
	if *scrollOffset < 0 {
		*scrollOffset = 0
	}

	var buf bytes.Buffer

	// Hide cursor and move to home position
	buf.WriteString("\033[?25l\033[H")

	// Header line 1
	statusTag := "\033[32m● LIVE\033[0m"
	if paused {
		statusTag = "\033[33m⏸ PAUSED\033[0m"
	}
	title := fmt.Sprintf("\033[1;36mMonsterMQ Live Topic Monitor\033[0m — %s", wsURL)
	buf.WriteString(fmt.Sprintf("%s  [%s]\033[K\r\n", title, statusTag))

	// Header line 2
	filtersStr := strings.Join(topicFilters, ", ")
	if len(filtersStr) > 40 {
		filtersStr = filtersStr[:37] + "..."
	}
	scrollInfo := fmt.Sprintf("%d-%d of %d", *scrollOffset+1, min(*scrollOffset+visibleRows, len(sortedKeys)), len(sortedKeys))
	if len(sortedKeys) == 0 {
		scrollInfo = "0 of 0"
	}
	buf.WriteString(fmt.Sprintf("Filters: \033[33m%s\033[0m | Topics: \033[1m%d\033[0m | View: %s | Sort: \033[1m%s\033[0m\033[K\r\n", filtersStr, len(sortedKeys), scrollInfo, sortBy))

	// Header line 3: Controls
	buf.WriteString("\033[90mControls: [↑/↓] Scroll  [s] Sort  [c] Clear  [p] Pause  [q] Quit\033[0m\033[K\r\n")

	// Table Header
	topicColWidth := 34
	valColWidth := width - topicColWidth - 22 - 6 - 6 - 8 - 6
	if valColWidth < 20 {
		valColWidth = 20
	}

	buf.WriteString(fmt.Sprintf("\033[1;37;44m%-*s %-*s %-20s %-4s %-4s %-6s\033[0m\033[K\r\n",
		topicColWidth, "TOPIC",
		valColWidth, "LATEST VALUE",
		"UPDATED (UTC)",
		"QOS",
		"RET",
		"UPD",
	))

	// Table Rows
	endIdx := min(*scrollOffset+visibleRows, len(sortedKeys))
	for i := *scrollOffset; i < endIdx; i++ {
		tName := sortedKeys[i]
		st := topicMap[tName]

		dispTopic := tName
		if len(dispTopic) > topicColWidth-1 {
			dispTopic = dispTopic[:topicColWidth-4] + "..."
		}

		dispVal := strings.ReplaceAll(st.Payload, "\n", " ")
		dispVal = strings.ReplaceAll(dispVal, "\r", "")
		if len(dispVal) > valColWidth-1 {
			dispVal = dispVal[:valColWidth-4] + "..."
		}

		timeStr := formatTimestampISO(st.Timestamp)
		if len(timeStr) > 19 {
			timeStr = timeStr[:19] + "Z"
		}

		retStr := "no"
		if st.Retained {
			retStr = "yes"
		}

		// Highlight recently updated (within 1 second)
		rowPrefix := ""
		if time.Since(st.LastUpdated) < 1*time.Second {
			rowPrefix = "\033[1;32m"
		}

		buf.WriteString(fmt.Sprintf("%s%-*s %-*s %-20s %-4d %-4s %-6d\033[0m\033[K\r\n",
			rowPrefix,
			topicColWidth, dispTopic,
			valColWidth, dispVal,
			timeStr,
			st.QoS,
			retStr,
			st.UpdateCount,
		))
	}

	// Pad blank rows to preserve height
	renderedRows := endIdx - *scrollOffset
	for i := renderedRows; i < visibleRows; i++ {
		buf.WriteString("\033[K\r\n")
	}

	// Clear remaining screen below
	buf.WriteString("\033[J")

	_, _ = os.Stdout.Write(buf.Bytes())
}

