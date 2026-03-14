package protocol_test

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"testing"

	"github.com/mukbeast4/go-mt5/internal/protocol"
)

func TestEncodeAndDecode(t *testing.T) {
	req := protocol.Request{
		ID:     "test-123",
		Action: "ping",
	}

	var buf bytes.Buffer
	if err := protocol.Encode(&buf, &req); err != nil {
		t.Fatalf("encode: %v", err)
	}

	data := buf.Bytes()
	if len(data) < 4 {
		t.Fatal("encoded data too short")
	}

	size := binary.BigEndian.Uint32(data[:4])
	payload := data[4:]
	if int(size) != len(payload) {
		t.Errorf("size mismatch: header says %d, got %d bytes", size, len(payload))
	}

	var decoded protocol.Request
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.ID != "test-123" {
		t.Errorf("expected id test-123, got %s", decoded.ID)
	}
	if decoded.Action != "ping" {
		t.Errorf("expected action ping, got %s", decoded.Action)
	}
}

func TestDecodeResponse(t *testing.T) {
	resp := protocol.Response{
		ID:      "resp-456",
		Type:    protocol.TypeResponse,
		Success: true,
		Data:    json.RawMessage(`{"key":"value"}`),
	}

	respData, _ := json.Marshal(resp)
	var buf bytes.Buffer
	header := make([]byte, 4)
	binary.BigEndian.PutUint32(header, uint32(len(respData)))
	buf.Write(header)
	buf.Write(respData)

	decoded, err := protocol.Decode(&buf)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if decoded.ID != "resp-456" {
		t.Errorf("expected id resp-456, got %s", decoded.ID)
	}
	if !decoded.Success {
		t.Error("expected success true")
	}
}

func TestEncodeBytes(t *testing.T) {
	req := protocol.Request{ID: "abc", Action: "test"}
	data, err := protocol.EncodeBytes(&req)
	if err != nil {
		t.Fatalf("encode bytes: %v", err)
	}

	size := binary.BigEndian.Uint32(data[:4])
	if int(size) != len(data)-4 {
		t.Errorf("size mismatch: %d vs %d", size, len(data)-4)
	}
}

func TestDecodeOversizedMessage(t *testing.T) {
	var buf bytes.Buffer
	header := make([]byte, 4)
	binary.BigEndian.PutUint32(header, 100*1024*1024)
	buf.Write(header)

	_, err := protocol.Decode(&buf)
	if err == nil {
		t.Fatal("expected error for oversized message")
	}
}
