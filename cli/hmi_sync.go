package main

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"
	"github.com/fsnotify/fsnotify"
)

// HmiSyncRequest matches the upstream MQTT sync protocol envelope.
type HmiSyncRequest struct {
	Action        string `json:"action"`
	ReqID         string `json:"reqId"`
	Dashboard     string `json:"dashboard,omitempty"`
	Path          string `json:"path,omitempty"`
	ContentBase64 string `json:"contentBase64,omitempty"`
	SHA256        string `json:"sha256,omitempty"`
	ZipBase64     string `json:"zipBase64,omitempty"`
	SetAsMain     bool   `json:"setAsMain,omitempty"`
}

// HmiSyncFileEntry represents a file in a dashboard listing.
type HmiSyncFileEntry struct {
	Path      string `json:"path"`
	SizeBytes int64  `json:"sizeBytes"`
	SHA256    string `json:"sha256,omitempty"`
	ModTime   int64  `json:"modTime,omitempty"`
}

// HmiSyncResponse matches the downstream MQTT sync protocol envelope.
type HmiSyncResponse struct {
	Action        string             `json:"action"`
	ReqID         string             `json:"reqId"`
	Success       bool               `json:"success"`
	Error         string             `json:"error,omitempty"`
	Dashboard     string             `json:"dashboard,omitempty"`
	Path          string             `json:"path,omitempty"`
	BytesWritten  int64              `json:"bytesWritten,omitempty"`
	ZipBase64     string             `json:"zipBase64,omitempty"`
	ContentBase64 string             `json:"contentBase64,omitempty"`
	SHA256        string             `json:"sha256,omitempty"`
	Files         []HmiSyncFileEntry `json:"files,omitempty"`
	FileCount     int                `json:"fileCount,omitempty"`
	SizeBytes     int64              `json:"sizeBytes,omitempty"`
	NodeID        string             `json:"nodeId,omitempty"`
	BrokerVersion string             `json:"brokerVersion,omitempty"`
	MainDashboard string             `json:"mainDashboard,omitempty"`
	Dashboards    []string           `json:"dashboards,omitempty"`
}

func generateSessionUUID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return fmt.Sprintf("sess_%d_%s", time.Now().Unix(), hex.EncodeToString(b))
}

func runHmiSync(ctx context.Context, client *Client, args []string) error {
	if len(args) == 0 || hasHelpFlag(args) {
		fmt.Println("Usage: mmq hmi sync <dashboard> [local-dir] [options]")
		fmt.Println()
		fmt.Println("Synchronize HMI dashboard files between local workstation and remote broker via MQTT.")
		fmt.Println("Cross-platform file watcher pushes edits live to the broker's HMI directory.")
		fmt.Println()
		fmt.Println("Arguments:")
		fmt.Println("  <dashboard>              Target HMI dashboard name (e.g. 'main')")
		fmt.Println("  [local-dir]              Local directory to synchronize (default: ./<dashboard>)")
		fmt.Println()
		fmt.Println("Options:")
		fmt.Println("  --pull                   Pull remote dashboard files to local directory before watching")
		fmt.Println("  --pull-only              Pull remote dashboard files once to local directory and exit")
		fmt.Println("  --push-only              Push all local directory files once to remote broker and exit")
		fmt.Println("  --mqtt-host <host>       MQTT broker host (default: derived from GraphQL host)")
		fmt.Println("  --mqtt-port <port>       MQTT broker port (default: 1883 or 8883 for TLS)")
		fmt.Println("  --mqtt-user <username>   MQTT username (default: CLI auth user)")
		fmt.Println("  --mqtt-pass <password>   MQTT password (default: CLI auth pass)")
		fmt.Println("  --base-topic <topic>     Base topic prefix (default: monstermq/hmi/sync)")
		fmt.Println("  --debounce <ms>          Debounce delay in ms for file write events (default: 100)")
		fmt.Println("  --ignore <patterns>      Comma-separated ignore patterns (default: .git,node_modules,*.tmp)")
		fmt.Println("  --dry-run                Log actions without transmitting MQTT packets")
		fmt.Println("  -h, --help               Show this help text")
		return nil
	}

	var dashboard, localDir string
	var pullFlag, pullOnlyFlag, pushOnlyFlag, dryRun bool
	var mqttHost, mqttUser, mqttPass, baseTopic string
	mqttPort := 0
	debounceMs := 100
	ignorePatterns := []string{".git", "node_modules", ".DS_Store", "*.tmp", "*~"}

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--pull":
			pullFlag = true
		case "--pull-only":
			pullOnlyFlag = true
		case "--push-only":
			pushOnlyFlag = true
		case "--dry-run":
			dryRun = true
		case "--mqtt-host":
			if i+1 < len(args) {
				mqttHost = args[i+1]
				i++
			}
		case "--mqtt-port":
			if i+1 < len(args) {
				mqttPort, _ = strconv.Atoi(args[i+1])
				i++
			}
		case "--mqtt-user":
			if i+1 < len(args) {
				mqttUser = args[i+1]
				i++
			}
		case "--mqtt-pass":
			if i+1 < len(args) {
				mqttPass = args[i+1]
				i++
			}
		case "--base-topic":
			if i+1 < len(args) {
				baseTopic = args[i+1]
				i++
			}
		case "--debounce":
			if i+1 < len(args) {
				if d, err := strconv.Atoi(args[i+1]); err == nil && d > 0 {
					debounceMs = d
				}
				i++
			}
		case "--ignore":
			if i+1 < len(args) {
				parts := strings.Split(args[i+1], ",")
				for _, p := range parts {
					if trimmed := strings.TrimSpace(p); trimmed != "" {
						ignorePatterns = append(ignorePatterns, trimmed)
					}
				}
				i++
			}
		default:
			if !strings.HasPrefix(arg, "-") {
				if dashboard == "" {
					dashboard = arg
				} else if localDir == "" {
					localDir = arg
				}
			}
		}
	}

	if dashboard == "" {
		dashboard = "main"
	}
	if localDir == "" {
		localDir = dashboard
	}
	if baseTopic == "" {
		baseTopic = "monstermq/hmi/sync"
	}
	baseTopic = strings.TrimRight(baseTopic, "/")

	// Resolve MQTT host and port from client config if not explicitly set
	if mqttHost == "" && client != nil && client.cfg.MqttHost != "" {
		mqttHost = client.cfg.MqttHost
	}
	if mqttHost == "" {
		if client != nil {
			if parsedURL, err := url.Parse(client.cfg.URL); err == nil && parsedURL.Hostname() != "" {
				mqttHost = parsedURL.Hostname()
			}
		}
		if mqttHost == "" {
			mqttHost = "localhost"
		}
	}
	if mqttPort <= 0 && client != nil && client.cfg.MqttPort > 0 {
		mqttPort = client.cfg.MqttPort
	}
	// Auto-discover MQTT port from brokerConfig if not explicitly set
	if mqttPort <= 0 && client != nil {
		discCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		var cfgRes struct {
			Data struct {
				BrokerConfig struct {
					TCPPort  int `json:"tcpPort"`
					TcpsPort int `json:"tcpsPort"`
				} `json:"brokerConfig"`
			} `json:"data"`
		}
		if err := client.DoQuery(discCtx, `query { brokerConfig { tcpPort tcpsPort } }`, nil, &cfgRes); err == nil {
			if strings.HasPrefix(strings.ToLower(client.cfg.URL), "https://") && cfgRes.Data.BrokerConfig.TcpsPort > 0 {
				mqttPort = cfgRes.Data.BrokerConfig.TcpsPort
			} else if cfgRes.Data.BrokerConfig.TCPPort > 0 {
				mqttPort = cfgRes.Data.BrokerConfig.TCPPort
			}
		}
		cancel()
	}
	if mqttPort <= 0 {
		if client != nil && strings.HasPrefix(strings.ToLower(client.cfg.URL), "https://") {
			mqttPort = 8883
		} else {
			mqttPort = 1883
		}
	}
	if mqttUser == "" && client != nil {
		if client.cfg.MqttUser != "" {
			mqttUser = client.cfg.MqttUser
		} else {
			mqttUser = client.cfg.Username
		}
	}
	if mqttPass == "" && client != nil {
		if client.cfg.MqttPass != "" {
			mqttPass = client.cfg.MqttPass
		} else {
			mqttPass = client.cfg.Password
		}
	}

	sessionUUID := generateSessionUUID()
	brokerURI := fmt.Sprintf("tcp://%s:%d", mqttHost, mqttPort)
	if mqttPort == 8883 {
		brokerURI = fmt.Sprintf("ssl://%s:%d", mqttHost, mqttPort)
	}

	fmt.Printf("[HMI-SYNC] Starting session %s\n", sessionUUID)
	fmt.Printf("[HMI-SYNC] Broker:    %s\n", brokerURI)
	fmt.Printf("[HMI-SYNC] Dashboard: %s\n", dashboard)
	fmt.Printf("[HMI-SYNC] Local Dir: %s\n", localDir)

	// Ensure local directory exists
	if err := os.MkdirAll(localDir, 0755); err != nil {
		return fmt.Errorf("failed to create local directory %s: %w", localDir, err)
	}

	// Setup MQTT client
	mqttOpts := paho.NewClientOptions()
	mqttOpts.AddBroker(brokerURI)
	mqttOpts.SetClientID("mmq-sync-" + sessionUUID)
	mqttOpts.SetCleanSession(true)
	mqttOpts.SetAutoReconnect(true)
	mqttOpts.SetConnectTimeout(5 * time.Second)
	if mqttUser != "" {
		mqttOpts.SetUsername(mqttUser)
		mqttOpts.SetPassword(mqttPass)
	}

	downstreamTopic := fmt.Sprintf("%s/%s/downstream", baseTopic, sessionUUID)
	upstreamTopic := fmt.Sprintf("%s/%s/upstream", baseTopic, sessionUUID)

	var pendingMu sync.Mutex
	pending := make(map[string]chan HmiSyncResponse)
	var reqCounter uint64

	mqttOpts.SetDefaultPublishHandler(func(_ paho.Client, msg paho.Message) {
		var resp HmiSyncResponse
		if err := json.Unmarshal(msg.Payload(), &resp); err != nil {
			return
		}
		pendingMu.Lock()
		ch, ok := pending[resp.ReqID]
		pendingMu.Unlock()
		if ok && ch != nil {
			select {
			case ch <- resp:
			default:
			}
		}
	})

	mqttClient := paho.NewClient(mqttOpts)
	if token := mqttClient.Connect(); token.Wait() && token.Error() != nil {
		return fmt.Errorf("failed to connect to MQTT broker at %s: %w", brokerURI, token.Error())
	}
	defer mqttClient.Disconnect(500)

	// Subscribe to downstream response topic
	subToken := mqttClient.Subscribe(downstreamTopic, 1, nil)
	if subToken.Wait() && subToken.Error() != nil {
		return fmt.Errorf("failed to subscribe to downstream topic %s: %w", downstreamTopic, subToken.Error())
	}

	sendUpstream := func(req HmiSyncRequest) (*HmiSyncResponse, error) {
		req.ReqID = strconv.FormatUint(atomic.AddUint64(&reqCounter, 1), 10)
		ch := make(chan HmiSyncResponse, 1)

		pendingMu.Lock()
		pending[req.ReqID] = ch
		pendingMu.Unlock()

		defer func() {
			pendingMu.Lock()
			delete(pending, req.ReqID)
			pendingMu.Unlock()
		}()

		payload, err := json.Marshal(req)
		if err != nil {
			return nil, err
		}

		if dryRun && (req.Action == "write" || req.Action == "delete" || req.Action == "import") {
			fmt.Printf("[DRY-RUN] Upstream %s -> %s\n", req.Action, req.Path)
			return &HmiSyncResponse{Action: req.Action, ReqID: req.ReqID, Success: true}, nil
		}

		pubToken := mqttClient.Publish(upstreamTopic, 1, false, payload)
		if pubToken.Wait() && pubToken.Error() != nil {
			return nil, pubToken.Error()
		}

		select {
		case resp := <-ch:
			if !resp.Success {
				return &resp, fmt.Errorf("%s failed: %s", req.Action, resp.Error)
			}
			return &resp, nil
		case <-time.After(15 * time.Second):
			return nil, fmt.Errorf("timeout waiting for broker downstream response (reqId=%s, action=%s)", req.ReqID, req.Action)
		}
	}

	// 1. Verify broker connection with ping
	pingResp, err := sendUpstream(HmiSyncRequest{Action: "ping"})
	if err != nil {
		return fmt.Errorf("broker ping handshake failed: %w", err)
	}
	fmt.Printf("[CONNECTED] Broker NodeID=%s Version=%s MainDashboard=%s\n", pingResp.NodeID, pingResp.BrokerVersion, pingResp.MainDashboard)

	// Check if local directory is empty
	isLocalEmpty := true
	if entries, err := os.ReadDir(localDir); err == nil && len(entries) > 0 {
		isLocalEmpty = false
	}

	// 2. Initial Pull if requested or if directory is empty (and not push-only)
	if (pullFlag || pullOnlyFlag || isLocalEmpty) && !pushOnlyFlag {
		fmt.Printf("[PULL] Pulling dashboard %q into %s...\n", dashboard, localDir)
		exportResp, err := sendUpstream(HmiSyncRequest{
			Action:    "export",
			Dashboard: dashboard,
		})
		if err != nil {
			return fmt.Errorf("initial pull failed: %w", err)
		}

		zipBytes, err := base64.StdEncoding.DecodeString(exportResp.ZipBase64)
		if err != nil {
			return fmt.Errorf("invalid base64 zip payload: %w", err)
		}

		zipReader, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
		if err != nil {
			return fmt.Errorf("invalid zip archive from broker: %w", err)
		}

		extractedCount := 0
		for _, f := range zipReader.File {
			targetPath := filepath.Join(localDir, filepath.FromSlash(f.Name))
			// Verify lexical containment inside localDir
			rel, err := filepath.Rel(localDir, targetPath)
			if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				continue
			}

			if f.FileInfo().IsDir() {
				_ = os.MkdirAll(targetPath, 0755)
				continue
			}

			_ = os.MkdirAll(filepath.Dir(targetPath), 0755)
			outFile, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
			if err != nil {
				continue
			}
			rc, err := f.Open()
			if err != nil {
				outFile.Close()
				continue
			}
			_, _ = io.Copy(outFile, rc)
			rc.Close()
			outFile.Close()
			extractedCount++
		}
		fmt.Printf("[PULL] Extracted %d files (%d bytes) into %s\n", extractedCount, exportResp.SizeBytes, localDir)

		if pullOnlyFlag {
			fmt.Println("[PULL] Completed. Exiting (--pull-only).")
			return nil
		}
	}

	// Helper to check if a relative path should be ignored
	shouldIgnore := func(relPath string) bool {
		normalized := filepath.ToSlash(relPath)
		for _, pattern := range ignorePatterns {
			if strings.Contains(normalized, pattern) {
				return true
			}
			if matched, _ := filepath.Match(pattern, filepath.Base(normalized)); matched {
				return true
			}
		}
		return false
	}

	// 3. One-shot push if requested
	if pushOnlyFlag {
		fmt.Printf("[PUSH] Pushing local files from %s to remote dashboard %q...\n", localDir, dashboard)
		pushCount := 0
		var pushBytes int64

		err := filepath.Walk(localDir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			rel, err := filepath.Rel(localDir, path)
			if err != nil || shouldIgnore(rel) {
				return nil
			}

			data, err := os.ReadFile(path)
			if err != nil {
				return nil
			}

			h := sha256.Sum256(data)
			_, err = sendUpstream(HmiSyncRequest{
				Action:        "write",
				Dashboard:     dashboard,
				Path:          filepath.ToSlash(rel),
				ContentBase64: base64.StdEncoding.EncodeToString(data),
				SHA256:        hex.EncodeToString(h[:]),
			})
			if err != nil {
				fmt.Printf("  [ERROR] Failed to push %s: %v\n", rel, err)
			} else {
				pushCount++
				pushBytes += int64(len(data))
				fmt.Printf("  [PUSHED] %s (%d bytes)\n", filepath.ToSlash(rel), len(data))
			}
			return nil
		})
		if err != nil {
			return err
		}
		fmt.Printf("[PUSH] Successfully pushed %d files (%d bytes). Exiting (--push-only).\n", pushCount, pushBytes)
		return nil
	}

	// 4. Live Watch Mode
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create file watcher: %w", err)
	}
	defer watcher.Close()

	// Recursively add all subdirectories to watcher
	addWatchRecursive := func(root string) error {
		return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if info.IsDir() {
				rel, _ := filepath.Rel(localDir, path)
				if shouldIgnore(rel) {
					return filepath.SkipDir
				}
				_ = watcher.Add(path)
			}
			return nil
		})
	}

	if err := addWatchRecursive(localDir); err != nil {
		return fmt.Errorf("failed to watch %s: %w", localDir, err)
	}

	fmt.Println()
	fmt.Printf("[WATCH] Watching %s for changes (press Ctrl+C to stop)...\n", localDir)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	// Debounce map for writes: path -> timer
	var debounceMu sync.Mutex
	pendingWrites := make(map[string]time.Time)
	debounceTicker := time.NewTicker(time.Duration(debounceMs) * time.Millisecond)
	defer debounceTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			fmt.Println("\n[HMI-SYNC] Context cancelled, stopping...")
			return nil
		case <-sigCh:
			fmt.Println("\n[HMI-SYNC] Interrupt received, stopping...")
			return nil
		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}

			rel, err := filepath.Rel(localDir, event.Name)
			if err != nil || shouldIgnore(rel) {
				continue
			}

			normPath := filepath.ToSlash(rel)

			if event.Has(fsnotify.Create) {
				if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
					_ = addWatchRecursive(event.Name)
					continue
				}
			}

			if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) {
				debounceMu.Lock()
				pendingWrites[normPath] = time.Now()
				debounceMu.Unlock()
			} else if event.Has(fsnotify.Remove) || event.Has(fsnotify.Rename) {
				debounceMu.Lock()
				delete(pendingWrites, normPath)
				debounceMu.Unlock()

				// If file is gone, notify delete
				if _, err := os.Stat(event.Name); os.IsNotExist(err) {
					start := time.Now()
					_, err := sendUpstream(HmiSyncRequest{
						Action:    "delete",
						Dashboard: dashboard,
						Path:      normPath,
					})
					elapsed := time.Since(start).Milliseconds()
					if err != nil {
						fmt.Printf("[SYNC-ERR] Delete %s failed: %v\n", normPath, err)
					} else {
						fmt.Printf("[%s] [SYNC] Deleted %s -> OK (%dms)\n", time.Now().Format("15:04:05"), normPath, elapsed)
					}
				}
			}

		case <-debounceTicker.C:
			debounceMu.Lock()
			now := time.Now()
			var toSync []string
			for p, t := range pendingWrites {
				if now.Sub(t) >= time.Duration(debounceMs)*time.Millisecond {
					toSync = append(toSync, p)
					delete(pendingWrites, p)
				}
			}
			debounceMu.Unlock()

			for _, p := range toSync {
				diskPath := filepath.Join(localDir, filepath.FromSlash(p))
				data, err := os.ReadFile(diskPath)
				if err != nil {
					continue
				}

				h := sha256.Sum256(data)
				start := time.Now()
				_, err = sendUpstream(HmiSyncRequest{
					Action:        "write",
					Dashboard:     dashboard,
					Path:          p,
					ContentBase64: base64.StdEncoding.EncodeToString(data),
					SHA256:        hex.EncodeToString(h[:]),
				})
				elapsed := time.Since(start).Milliseconds()
				if err != nil {
					fmt.Printf("[SYNC-ERR] Write %s failed: %v\n", p, err)
				} else {
					fmt.Printf("[%s] [SYNC] Updated %s (%d bytes) -> OK (%dms)\n", time.Now().Format("15:04:05"), p, len(data), elapsed)
				}
			}

		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			fmt.Printf("[WATCH-ERR] %v\n", err)
		}
	}
}
