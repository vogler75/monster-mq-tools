package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestLoadDotEnv(t *testing.T) {
	tempDir := t.TempDir()
	envPath := filepath.Join(tempDir, ".env")

	content := `
# Sample config
MQ_URL=http://127.0.0.1:4000/graphql
MQ_USER=testuser
MQ_PASS="testpass"
`
	if err := os.WriteFile(envPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write temp env file: %v", err)
	}

	LoadDotEnv(envPath)

	if got := os.Getenv("MQ_URL"); got != "http://127.0.0.1:4000/graphql" {
		t.Errorf("expected MQ_URL=http://127.0.0.1:4000/graphql, got %s", got)
	}
	if got := os.Getenv("MQ_USER"); got != "testuser" {
		t.Errorf("expected MQ_USER=testuser, got %s", got)
	}
	if got := os.Getenv("MQ_PASS"); got != "testpass" {
		t.Errorf("expected MQ_PASS=testpass, got %s", got)
	}
}

func TestBuildEndpointURL(t *testing.T) {
	os.Unsetenv("MQ_URL")
	os.Unsetenv("GRAPHQL_URL")
	os.Unsetenv("MQ_HOST")
	os.Unsetenv("GRAPHQL_HOST")
	os.Unsetenv("MQ_PORT")
	os.Unsetenv("GRAPHQL_PORT")
	os.Unsetenv("MQ_HTTPS")
	os.Unsetenv("GRAPHQL_HTTPS")

	tests := []struct {
		name      string
		flagURL   string
		flagHost  string
		flagPort  int
		flagHTTPS bool
		want      string
	}{
		{
			name: "default",
			want: "http://localhost:4000/graphql",
		},
		{
			name:     "port only",
			flagPort: 4001,
			want:     "http://localhost:4001/graphql",
		},
		{
			name:     "host only",
			flagHost: "192.168.1.50",
			want:     "http://192.168.1.50:4000/graphql",
		},
		{
			name:      "host port https",
			flagHost:  "edge-broker",
			flagPort:  4001,
			flagHTTPS: true,
			want:      "https://edge-broker:4001/graphql",
		},
		{
			name:      "explicit url with https override",
			flagURL:   "http://custom-host:8080/graphql",
			flagHTTPS: true,
			want:      "https://custom-host:8080/graphql",
		},
		{
			name:     "explicit url with port override",
			flagURL:  "http://custom-host:8080/graphql",
			flagPort: 9090,
			want:     "http://custom-host:9090/graphql",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildEndpointURL(tt.flagURL, tt.flagHost, tt.flagPort, tt.flagHTTPS)
			if got != tt.want {
				t.Errorf("BuildEndpointURL() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestResolveClientConfig(t *testing.T) {
	cfg := ResolveClientConfig("http://custom:4000/graphql", "", 0, false, "admin", "secret", "tok123", "", true)

	if cfg.URL != "http://custom:4000/graphql" {
		t.Errorf("unexpected URL: %s", cfg.URL)
	}
	if cfg.Username != "admin" {
		t.Errorf("unexpected Username: %s", cfg.Username)
	}
	if cfg.Password != "secret" {
		t.Errorf("unexpected Password: %s", cfg.Password)
	}
	if cfg.Token != "tok123" {
		t.Errorf("unexpected Token: %s", cfg.Token)
	}
	if !cfg.JSONMode {
		t.Errorf("expected JSONMode to be true")
	}
}

func TestRunListFeatures(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":{"broker":{"enabledFeatures":["MqttClient","WinccUaBridge"]}}}`))
	}))
	defer server.Close()

	cfg := &ClientConfig{
		URL:     server.URL,
		Timeout: 5 * time.Second,
	}
	client := NewClient(cfg)

	err := runListFeatures(context.Background(), client, nil)
	if err != nil {
		t.Fatalf("runListFeatures failed: %v", err)
	}

	cfgJSON := &ClientConfig{
		URL:      server.URL,
		Timeout:  5 * time.Second,
		JSONMode: true,
	}
	clientJSON := NewClient(cfgJSON)

	err = runListFeatures(context.Background(), clientJSON, nil)
	if err != nil {
		t.Fatalf("runListFeatures json mode failed: %v", err)
	}
}

func TestSplitCommandLine(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []string
		wantErr bool
	}{
		{
			name:    "simple tokens",
			input:   "searchTopics sensors/#",
			want:    []string{"searchTopics", "sensors/#"},
			wantErr: false,
		},
		{
			name:    "single quoted payload",
			input:   `publish sensors/temp '{"temp": 22.5}' --retain`,
			want:    []string{"publish", "sensors/temp", `{"temp": 22.5}`, "--retain"},
			wantErr: false,
		},
		{
			name:    "double quoted string with spaces",
			input:   `searchTopics "Power Meters/*"`,
			want:    []string{"searchTopics", "Power Meters/*"},
			wantErr: false,
		},
		{
			name:    "escaped quotes",
			input:   `publish test/topic "hello \"world\""`,
			want:    []string{"publish", "test/topic", `hello "world"`},
			wantErr: false,
		},
		{
			name:    "unclosed single quote",
			input:   `publish test '{"temp": 22`,
			want:    nil,
			wantErr: true,
		},
		{
			name:    "unclosed double quote",
			input:   `searchTopics "sensors/`,
			want:    nil,
			wantErr: true,
		},
		{
			name:    "empty and whitespace",
			input:   "   ",
			want:    nil,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := splitCommandLine(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("splitCommandLine() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("splitCommandLine() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNormalizeEndpointURL(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"4001", "http://localhost:4001/graphql"},
		{":4001", "http://localhost:4001/graphql"},
		{"192.168.1.50:4001", "http://192.168.1.50:4001/graphql"},
		{"http://edge-node:4001/graphql", "http://edge-node:4001/graphql"},
		{"https://secure-node:4000", "https://secure-node:4000/graphql"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := normalizeEndpointURL(tt.input); got != tt.want {
				t.Errorf("normalizeEndpointURL(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestExecuteCommand(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":{"broker":{"enabledFeatures":["MqttClient"]}}}`))
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{URL: server.URL, Timeout: 5 * time.Second})
	ctx := context.Background()

	// Test valid command
	if err := ExecuteCommand(ctx, client, []string{"features"}); err != nil {
		t.Fatalf("ExecuteCommand(features) failed: %v", err)
	}

	// Test unknown command
	if err := ExecuteCommand(ctx, client, []string{"nonexistentCommand"}); err == nil {
		t.Fatalf("expected error for unknown command, got nil")
	}

	// Test empty command
	if err := ExecuteCommand(ctx, client, []string{}); err != nil {
		t.Fatalf("expected nil error for empty args, got %v", err)
	}
}

func TestProbeBrokerStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":{"broker":{"enabledFeatures":["MqttClient","WinccUaBridge"]},"currentUser":{"username":"admin","isAdmin":true}}}`))
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{URL: server.URL, Timeout: 5 * time.Second})
	ctx := context.Background()

	online, features, user, err := probeBrokerStatus(ctx, client)
	if err != nil {
		t.Fatalf("probeBrokerStatus failed: %v", err)
	}
	if !online {
		t.Errorf("expected online = true")
	}
	if len(features) != 2 || features[0] != "MqttClient" {
		t.Errorf("unexpected features: %v", features)
	}
	if user != "admin (Admin)" {
		t.Errorf("unexpected user: %s", user)
	}
}

func TestCompleter(t *testing.T) {
	completer := buildCompleter()
	if completer == nil {
		t.Fatalf("expected non-nil completer")
	}

	// Test "sea" -> "rchTopics "
	names, length := completer.Do([]rune("sea"), 3)
	if length != 3 {
		t.Errorf("expected length 3, got %d", length)
	}
	if len(names) == 0 {
		t.Errorf("expected candidates for 'sea', got 0")
	}
	foundSearchTopics := false
	for _, n := range names {
		if string(n) == "rchTopics " || string(n) == "searchTopics " || string(n) == "rchTopics" {
			foundSearchTopics = true
		}
	}
	if !foundSearchTopics {
		t.Errorf("expected searchTopics completion, got %v", names)
	}

	// Test "s" -> multiple candidates starting with 's' (searchTopics, session, sessions, status, set-value, etc.)
	sNames, _ := completer.Do([]rune("s"), 1)
	if len(sNames) < 3 {
		t.Errorf("expected multiple candidates for 's', got %d", len(sNames))
	}
}

func TestRunQueryHistory(t *testing.T) {
	var capturedVars map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		var req struct {
			Variables map[string]any `json:"variables"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		capturedVars = req.Variables

		_, _ = w.Write([]byte(`{"data":{"archivedMessages":[{"topic":"sensors/temp/room1","payload":"{\"temp\":22.5}","format":"JSON","timestamp":1700000000,"qos":0,"clientId":"client1"}]}}`))
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{URL: server.URL, Timeout: 5 * time.Second})
	ctx := context.Background()

	// 1. Test default: no timerange specified -> defaults to 60s window, archiveGroup: "Default"
	err := runQueryHistory(ctx, client, []string{"sensors/temp/room1"})
	if err != nil {
		t.Fatalf("runQueryHistory failed: %v", err)
	}
	if capturedVars["archiveGroup"] != "Default" {
		t.Errorf("expected archiveGroup Default, got %v", capturedVars["archiveGroup"])
	}
	if capturedVars["startTime"] == nil || capturedVars["startTime"] == "" {
		t.Errorf("expected default startTime to be populated with 60s window, got %v", capturedVars["startTime"])
	}
	if capturedVars["endTime"] == nil || capturedVars["endTime"] == "" {
		t.Errorf("expected default endTime to be populated, got %v", capturedVars["endTime"])
	}

	// 2. Test positional archive group: "CustomGroup"
	err = runQueryHistory(ctx, client, []string{"sensors/temp/room1", "CustomGroup"})
	if err != nil {
		t.Fatalf("runQueryHistory positional group failed: %v", err)
	}
	if capturedVars["archiveGroup"] != "CustomGroup" {
		t.Errorf("expected archiveGroup CustomGroup, got %v", capturedVars["archiveGroup"])
	}

	// 3. Test explicit flag: --archive-group FlagGroup
	err = runQueryHistory(ctx, client, []string{"sensors/temp/room1", "--archive-group", "FlagGroup"})
	if err != nil {
		t.Fatalf("runQueryHistory flag group failed: %v", err)
	}
	if capturedVars["archiveGroup"] != "FlagGroup" {
		t.Errorf("expected archiveGroup FlagGroup, got %v", capturedVars["archiveGroup"])
	}
}

func TestRunGetValue(t *testing.T) {
	var capturedVars map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		var req struct {
			Variables map[string]any `json:"variables"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		capturedVars = req.Variables

		_, _ = w.Write([]byte(`{"data":{"currentValue":{"topic":"test","payload":"SGVsbG8gV29ybGQgMw==","format":"BINARY","timestamp":1786683732069,"qos":0},"retainedMessage":null}}`))
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{URL: server.URL, Timeout: 5 * time.Second})
	ctx := context.Background()

	// 1. Test basic currentValue with positional archive group
	err := runGetValue(ctx, client, []string{"test", "pg1"})
	if err != nil {
		t.Fatalf("runGetValue failed: %v", err)
	}
	if capturedVars["archiveGroup"] != "pg1" {
		t.Errorf("expected archiveGroup pg1, got %v", capturedVars["archiveGroup"])
	}

	// 2. Test formatPayloadAndType helper directly
	str, typ := formatPayloadAndType("SGVsbG8gV29ybGQgMw==", "BINARY")
	if str != "Hello World 3" {
		t.Errorf("expected decoded payload 'Hello World 3', got '%s'", str)
	}
	if typ != "STRING" {
		t.Errorf("expected detected type 'STRING', got '%s'", typ)
	}

	// 3. Test timestamp helper
	iso := formatTimestampISO(1786683732069)
	if !strings.Contains(iso, "T") || !strings.HasSuffix(iso, "Z") {
		t.Errorf("expected valid ISO 8601 timestamp, got '%s'", iso)
	}
}

func TestRunSetValue(t *testing.T) {
	var capturedVars map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		var req struct {
			Variables map[string]any `json:"variables"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		capturedVars = req.Variables

		_, _ = w.Write([]byte(`{"data":{"publish":{"success":true,"error":null,"topic":"sensors/temp","timestamp":1786683732069}}}`))
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{URL: server.URL, Timeout: 5 * time.Second})
	ctx := context.Background()

	err := runSetValue(ctx, client, []string{"sensors/temp", `{"val": 25.5}`, "--retain", "--qos", "1"})
	if err != nil {
		t.Fatalf("runSetValue failed: %v", err)
	}
	if capturedVars["topic"] != "sensors/temp" {
		t.Errorf("expected topic sensors/temp, got %v", capturedVars["topic"])
	}
	if capturedVars["retained"] != true {
		t.Errorf("expected retained true, got %v", capturedVars["retained"])
	}
	if capturedVars["qos"] != float64(1) {
		t.Errorf("expected qos 1, got %v", capturedVars["qos"])
	}
}

func TestCommandHelpFlags(t *testing.T) {
	client := NewClient(&ClientConfig{URL: "http://localhost:4000/graphql", Timeout: 5 * time.Second})
	ctx := context.Background()

	// Missing args or -h / --help must return nil and display usage instead of error
	if err := runQueryHistory(ctx, client, []string{}); err != nil {
		t.Errorf("expected nil error for empty args on archivedMessages, got %v", err)
	}
	if err := runQueryHistory(ctx, client, []string{"-h"}); err != nil {
		t.Errorf("expected nil error for -h on archivedMessages, got %v", err)
	}
	if err := runGetValue(ctx, client, []string{}); err != nil {
		t.Errorf("expected nil error for empty args on currentValue, got %v", err)
	}
	if err := runGetValue(ctx, client, []string{"-h"}); err != nil {
		t.Errorf("expected nil error for -h on currentValue, got %v", err)
	}
	if err := runSetValue(ctx, client, []string{"-h"}); err != nil {
		t.Errorf("expected nil error for -h on publish, got %v", err)
	}
	if err := runListTopics(ctx, client, []string{"--help"}); err != nil {
		t.Errorf("expected nil error for --help on searchTopics, got %v", err)
	}
	if err := runDatabaseConnections(ctx, client, []string{"-h"}); err != nil {
		t.Errorf("expected nil error for -h on databaseConnections, got %v", err)
	}
}

func TestRunDatabaseConnections(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		_, _ = w.Write([]byte(`{"data":{"databaseConnections":[{"name":"pg1","type":"POSTGRES","url":"postgres://localhost:5432/db","database":"db","schema":"public","readOnly":false}]}}`))
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{URL: server.URL, Timeout: 5 * time.Second})
	ctx := context.Background()

	err := runDatabaseConnections(ctx, client, nil)
	if err != nil {
		t.Fatalf("runDatabaseConnections failed: %v", err)
	}
}

func TestRunHmiList(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		_, _ = w.Write([]byte(`{"data":{"hmis":[{"name":"factory-hmi","nodeId":"node1","enabled":true,"isOnCurrentNode":true,"config":{"urlPath":"/factory","isMain":true,"title":"Factory Overview","description":"Main plant HMI","entryPoint":"index.html"},"fileCount":10,"sizeBytes":2048}]}}`))
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{URL: server.URL, Timeout: 5 * time.Second})
	ctx := context.Background()

	err := runHmiList(ctx, client, nil)
	if err != nil {
		t.Fatalf("runHmiList failed: %v", err)
	}
}

func TestRunSessionDirectAndHelp(t *testing.T) {
	var capturedVars map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		var req struct {
			Variables map[string]any `json:"variables"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		capturedVars = req.Variables

		_, _ = w.Write([]byte(`{"data":{"session":{"clientId":"device-123","nodeId":"node1","connected":true,"cleanSession":true,"clientAddress":"127.0.0.1","protocolVersion":4,"queuedMessageCount":0,"subscriptions":[{"topicFilter":"sensors/#","qos":1}]}}}`))
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{URL: server.URL, Timeout: 5 * time.Second})
	ctx := context.Background()

	// 1. Direct session <clientId> syntax
	err := ExecuteCommand(ctx, client, []string{"session", "device-123"})
	if err != nil {
		t.Fatalf("ExecuteCommand(session device-123) failed: %v", err)
	}
	if capturedVars["clientId"] != "device-123" {
		t.Errorf("expected clientId device-123, got %v", capturedVars["clientId"])
	}

	// 2. session -h and sessions -h
	if err := ExecuteCommand(ctx, client, []string{"session", "-h"}); err != nil {
		t.Errorf("expected nil error for session -h, got %v", err)
	}
	if err := ExecuteCommand(ctx, client, []string{"sessions", "-h"}); err != nil {
		t.Errorf("expected nil error for sessions -h, got %v", err)
	}
}

func TestRunExportHmiZip(t *testing.T) {
	tempDir := t.TempDir()
	outZipPath := filepath.Join(tempDir, "my-test-hmi.zip")
	expectedContent := "sample-binary-zip-payload-data"
	b64Data := base64.StdEncoding.EncodeToString([]byte(expectedContent))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		_, _ = w.Write([]byte(fmt.Sprintf(`{"data":{"exportHmiZip":"%s"}}`, b64Data)))
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{URL: server.URL, Timeout: 5 * time.Second})
	ctx := context.Background()

	// 1. Test help flag
	if err := runExportHmiZip(ctx, client, []string{"-h"}); err != nil {
		t.Errorf("expected nil error for -h, got %v", err)
	}

	// 2. Test export with target output path
	err := runExportHmiZip(ctx, client, []string{"my-test-hmi", outZipPath})
	if err != nil {
		t.Fatalf("runExportHmiZip failed: %v", err)
	}

	// Verify file was written
	data, err := os.ReadFile(outZipPath)
	if err != nil {
		t.Fatalf("failed to read written zip file: %v", err)
	}
	if string(data) != expectedContent {
		t.Errorf("unexpected content in zip: got %q, want %q", string(data), expectedContent)
	}
}

func TestRunImportHmiZip(t *testing.T) {
	tempDir := t.TempDir()
	sampleZipPath := filepath.Join(tempDir, "plant-dashboard.zip")
	_ = os.WriteFile(sampleZipPath, []byte("fake-zip-binary-data"), 0644)

	var capturedVars map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		var req struct {
			Variables map[string]any `json:"variables"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		capturedVars = req.Variables

		_, _ = w.Write([]byte(`{"data":{"hmi":{"uploadZip":{"success":true,"message":"OK","hmi":{"name":"plant-dashboard","nodeId":"node1","enabled":true,"config":{"urlPath":"/plant-dashboard","isMain":true,"title":"Plant Dashboard"}}}}}}`))
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{URL: server.URL, Timeout: 5 * time.Second})
	ctx := context.Background()

	// 1. Help flag
	if err := runImportHmiZip(ctx, client, []string{"-h"}); err != nil {
		t.Errorf("expected nil error for -h, got %v", err)
	}

	// 2. Import zip file with --main flag
	err := ExecuteCommand(ctx, client, []string{"importHmiZip", sampleZipPath, "--main"})
	if err != nil {
		t.Fatalf("runImportHmiZip failed: %v", err)
	}

	if capturedVars["name"] != "plant-dashboard" {
		t.Errorf("expected name 'plant-dashboard', got %v", capturedVars["name"])
	}
	if capturedVars["setAsMain"] != true {
		t.Errorf("expected setAsMain true, got %v", capturedVars["setAsMain"])
	}
	if capturedVars["zipBase64"] != base64.StdEncoding.EncodeToString([]byte("fake-zip-binary-data")) {
		t.Errorf("unexpected zipBase64: %v", capturedVars["zipBase64"])
	}
}

func TestRunDeviceListFilter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		_, _ = w.Write([]byte(`{"data":{"getDevices":[
			{"name":"opc1","namespace":"ns1","nodeId":"local","type":"OPCUA_CLIENT","enabled":true,"createdAt":"2026-08-14","updatedAt":"2026-08-14"},
			{"name":"mqtt1","namespace":"ns2","nodeId":"local","type":"MQTT_CLIENT","enabled":true,"createdAt":"2026-08-14","updatedAt":"2026-08-14"}
		]}}`))
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{URL: server.URL, Timeout: 5 * time.Second})
	ctx := context.Background()

	// 1. Help flag
	if err := runDeviceList(ctx, client, []string{"-h"}); err != nil {
		t.Errorf("expected nil error for -h, got %v", err)
	}

	// 2. Filter positional
	if err := runDeviceList(ctx, client, []string{"OPCUA_CLIENT"}); err != nil {
		t.Fatalf("runDeviceList(OPCUA_CLIENT) failed: %v", err)
	}

	// 3. Filter with --type flag
	if err := runDeviceList(ctx, client, []string{"--type", "mqtt"}); err != nil {
		t.Fatalf("runDeviceList(--type mqtt) failed: %v", err)
	}
}

func TestRunHmiCreateAndRemove(t *testing.T) {
	var capturedVars map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		var req struct {
			Query     string         `json:"query"`
			Variables map[string]any `json:"variables"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		capturedVars = req.Variables

		if strings.Contains(req.Query, "CreateHmi") {
			_, _ = w.Write([]byte(`{"data":{"hmi":{"create":{"success":true,"message":"Created","hmi":{"name":"new-hmi","nodeId":"local","enabled":true,"config":{"urlPath":"/new-hmi","isMain":true,"title":"New HMI"}}}}}}`))
		} else {
			_, _ = w.Write([]byte(`{"data":{"hmi":{"delete":{"success":true,"message":"Deleted"}}}}`))
		}
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{URL: server.URL, Timeout: 5 * time.Second})
	ctx := context.Background()

	// 1. Test create help
	if err := runHmiCreate(ctx, client, []string{"-h"}); err != nil {
		t.Errorf("expected nil error for create -h, got %v", err)
	}

	// 2. Test create execution
	err := ExecuteCommand(ctx, client, []string{"hmi", "create", "new-hmi", "--title", "New HMI", "--main"})
	if err != nil {
		t.Fatalf("hmi create failed: %v", err)
	}
	inputMap, ok := capturedVars["input"].(map[string]any)
	if !ok || inputMap["name"] != "new-hmi" {
		t.Errorf("expected input.name new-hmi, got %v", capturedVars["input"])
	}

	// 3. Test remove help
	if err := runHmiRemove(ctx, client, []string{"-h"}); err != nil {
		t.Errorf("expected nil error for remove -h, got %v", err)
	}

	// 4. Test remove execution
	err = ExecuteCommand(ctx, client, []string{"hmi", "remove", "new-hmi"})
	if err != nil {
		t.Fatalf("hmi remove failed: %v", err)
	}
	if capturedVars["name"] != "new-hmi" {
		t.Errorf("expected name 'new-hmi', got %v", capturedVars["name"])
	}
}

func TestRunImportHmiZipFromDirectory(t *testing.T) {
	tempDir := t.TempDir()
	hmiSrcDir := filepath.Join(tempDir, "sample-hmi-dir")
	_ = os.MkdirAll(filepath.Join(hmiSrcDir, "css"), 0755)
	_ = os.WriteFile(filepath.Join(hmiSrcDir, "index.html"), []byte("<h1>Hello HMI</h1>"), 0644)
	_ = os.WriteFile(filepath.Join(hmiSrcDir, "css", "style.css"), []byte("body { color: red; }"), 0644)

	var capturedVars map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		var req struct {
			Variables map[string]any `json:"variables"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		capturedVars = req.Variables

		_, _ = w.Write([]byte(`{"data":{"hmi":{"uploadZip":{"success":true,"message":"OK","hmi":{"name":"sample-hmi-dir","nodeId":"local","enabled":true,"config":{"urlPath":"/sample-hmi-dir","isMain":false,"title":"sample-hmi-dir"}}}}}}`))
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{URL: server.URL, Timeout: 5 * time.Second})
	ctx := context.Background()

	// Upload directory directly
	err := ExecuteCommand(ctx, client, []string{"importHmiZip", hmiSrcDir})
	if err != nil {
		t.Fatalf("importHmiZip on directory failed: %v", err)
	}

	if capturedVars["name"] != "sample-hmi-dir" {
		t.Errorf("expected auto-detected name 'sample-hmi-dir', got %v", capturedVars["name"])
	}

	zipB64, ok := capturedVars["zipBase64"].(string)
	if !ok || zipB64 == "" {
		t.Fatalf("expected non-empty zipBase64")
	}

	// Verify the generated zip contains the files
	zipBytes, err := base64.StdEncoding.DecodeString(zipB64)
	if err != nil {
		t.Fatalf("failed to decode base64: %v", err)
	}

	zr, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		t.Fatalf("failed to read zip payload: %v", err)
	}

	fileMap := make(map[string]bool)
	for _, f := range zr.File {
		fileMap[f.Name] = true
	}
	if !fileMap["index.html"] {
		t.Errorf("expected index.html in zip, got %v", fileMap)
	}
	if !fileMap["css/style.css"] {
		t.Errorf("expected css/style.css in zip, got %v", fileMap)
	}
}

func TestRunExportHmiZipUnzip(t *testing.T) {
	tempDir := t.TempDir()
	outExtractDir := filepath.Join(tempDir, "extracted-hmi")

	// Create in-memory zip
	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)
	w, _ := zw.Create("index.html")
	_, _ = w.Write([]byte("<h1>Dashboard Content</h1>"))
	w2, _ := zw.Create("app.js")
	_, _ = w2.Write([]byte("console.log('running');"))
	_ = zw.Close()

	zipB64 := base64.StdEncoding.EncodeToString(buf.Bytes())

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		_, _ = w.Write([]byte(fmt.Sprintf(`{"data":{"exportHmiZip":"%s"}}`, zipB64)))
	}))
	defer server.Close()

	client := NewClient(&ClientConfig{URL: server.URL, Timeout: 5 * time.Second})
	ctx := context.Background()

	// Export with --unzip to directory
	err := ExecuteCommand(ctx, client, []string{"exportHmiZip", "my-dash", outExtractDir, "--unzip"})
	if err != nil {
		t.Fatalf("exportHmiZip --unzip failed: %v", err)
	}

	// Verify extracted files exist
	htmlContent, err := os.ReadFile(filepath.Join(outExtractDir, "index.html"))
	if err != nil {
		t.Fatalf("failed to read extracted index.html: %v", err)
	}
	if string(htmlContent) != "<h1>Dashboard Content</h1>" {
		t.Errorf("unexpected index.html content: %s", string(htmlContent))
	}

	jsContent, err := os.ReadFile(filepath.Join(outExtractDir, "app.js"))
	if err != nil {
		t.Fatalf("failed to read extracted app.js: %v", err)
	}
	if string(jsContent) != "console.log('running');" {
		t.Errorf("unexpected app.js content: %s", string(jsContent))
	}
}












