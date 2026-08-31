package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMCPClientInitializeAndCallTool(t *testing.T) {
	const sessionID = "test-session"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("Authorization header = %q", got)
		}
		var request rpcRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode request: %v", err)
		}

		switch request.Method {
		case "initialize":
			w.Header().Set("Mcp-Session-Id", sessionID)
			writeRPCResult(w, request.ID, map[string]any{
				"protocolVersion": protocolVersion,
				"serverInfo":      map[string]string{"name": "test-server"},
			})
		case "notifications/initialized":
			if got := r.Header.Get("Mcp-Session-Id"); got != sessionID {
				t.Errorf("notification session = %q", got)
			}
			w.WriteHeader(http.StatusAccepted)
		case "tools/call":
			if got := r.Header.Get("Mcp-Session-Id"); got != sessionID {
				t.Errorf("tool-call session = %q", got)
			}
			writeRPCResult(w, request.ID, map[string]any{
				"isError": false,
				"content": []map[string]string{{"type": "text", "text": `{"ok":true}`}},
			})
		default:
			t.Errorf("unexpected MCP method %q", request.Method)
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer server.Close()

	client := newMCPClient(config{endpoint: server.URL, apiKey: "test-key"})
	ctx := context.Background()
	if _, err := client.initialize(ctx); err != nil {
		t.Fatalf("initialize: %v", err)
	}
	if client.session != sessionID {
		t.Fatalf("session = %q, want %q", client.session, sessionID)
	}

	result, err := client.callTool(ctx, "test_tool", map[string]any{"symbol": "EURUSD"})
	if err != nil {
		t.Fatalf("callTool: %v", err)
	}
	var decoded toolCallResult
	if err := json.Unmarshal(result, &decoded); err != nil {
		t.Fatalf("decode tool result: %v", err)
	}
	if len(decoded.Content) != 1 || decoded.Content[0].Text != `{"ok":true}` {
		t.Fatalf("unexpected tool result: %+v", decoded)
	}
}

func TestExtractJSONFromEventStream(t *testing.T) {
	got, err := extractJSON([]byte("event: message\ndata: {\"jsonrpc\":\"2.0\"}\n\n"), "text/event-stream")
	if err != nil {
		t.Fatalf("extractJSON: %v", err)
	}
	if string(got) != `{"jsonrpc":"2.0"}` {
		t.Fatalf("JSON = %s", got)
	}
}

func writeRPCResult(w http.ResponseWriter, id *int64, result any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"result":  result,
	})
}
