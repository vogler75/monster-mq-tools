package main

import (
	"context"
	"strings"
	"testing"
)

func TestHmiSyncHelpAndUUID(t *testing.T) {
	uuid1 := generateSessionUUID()
	uuid2 := generateSessionUUID()
	if uuid1 == "" || uuid2 == "" || uuid1 == uuid2 {
		t.Fatalf("expected unique non-empty session UUIDs: %s, %s", uuid1, uuid2)
	}
	if !strings.HasPrefix(uuid1, "sess_") {
		t.Fatalf("expected sess_ prefix, got: %s", uuid1)
	}

	client := NewClient(&ClientConfig{
		URL:      "http://localhost:4000/graphql",
		Username: "globaluser",
		Password: "globalpass",
		MqttHost: "192.168.1.100",
		MqttPort: 1884,
		MqttUser: "mqttuser",
		MqttPass: "mqttpass",
	})
	// Test help
	err := runHmiSync(context.Background(), client, []string{"--help"})
	if err != nil {
		t.Fatalf("expected no error for --help, got: %v", err)
	}

	if client.cfg.MqttUser != "mqttuser" || client.cfg.MqttPass != "mqttpass" {
		t.Fatalf("expected mqtt credentials, got: %s / %s", client.cfg.MqttUser, client.cfg.MqttPass)
	}
	if client.cfg.MqttHost != "192.168.1.100" || client.cfg.MqttPort != 1884 {
		t.Fatalf("expected mqtt host/port, got: %s / %d", client.cfg.MqttHost, client.cfg.MqttPort)
	}
}
