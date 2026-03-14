package protocol_test

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/mukbeast4/go-mt5/internal/protocol"
)

func TestWriterU32(t *testing.T) {
	w := protocol.NewWriter()
	w.WriteU32(42)
	data := w.Bytes()
	if len(data) != 4 {
		t.Fatalf("expected 4 bytes, got %d", len(data))
	}
	v := binary.LittleEndian.Uint32(data)
	if v != 42 {
		t.Errorf("expected 42, got %d", v)
	}
}

func TestWriterF64(t *testing.T) {
	w := protocol.NewWriter()
	w.WriteF64(1.0850)
	data := w.Bytes()
	if len(data) != 8 {
		t.Fatalf("expected 8 bytes, got %d", len(data))
	}
	r := protocol.NewReader(data)
	v := r.ReadF64()
	if v != 1.0850 {
		t.Errorf("expected 1.0850, got %f", v)
	}
}

func TestWriterString(t *testing.T) {
	w := protocol.NewWriter()
	w.WriteString("Python")
	data := w.Bytes()

	charCount := binary.LittleEndian.Uint32(data[0:4])
	if charCount != 6 {
		t.Errorf("expected char count 6, got %d", charCount)
	}
	if len(data) != 4+6*2 {
		t.Errorf("expected %d bytes, got %d", 4+6*2, len(data))
	}

	r := protocol.NewReader(data)
	s := r.ReadString()
	if s != "Python" {
		t.Errorf("expected 'Python', got '%s'", s)
	}
}

func TestReaderSequential(t *testing.T) {
	w := protocol.NewWriter()
	w.WriteU32(100)
	w.WriteI64(-5)
	w.WriteF64(3.14)
	w.WriteString("test")

	r := protocol.NewReader(w.Bytes())

	if v := r.ReadU32(); v != 100 {
		t.Errorf("u32: expected 100, got %d", v)
	}
	if v := r.ReadI64(); v != -5 {
		t.Errorf("i64: expected -5, got %d", v)
	}
	if v := r.ReadF64(); v != 3.14 {
		t.Errorf("f64: expected 3.14, got %f", v)
	}
	if v := r.ReadString(); v != "test" {
		t.Errorf("string: expected 'test', got '%s'", v)
	}
	if r.Err() != nil {
		t.Errorf("unexpected error: %v", r.Err())
	}
}

func TestReaderOverflow(t *testing.T) {
	r := protocol.NewReader([]byte{0x01, 0x02})
	r.ReadU32()
	if r.Err() == nil {
		t.Error("expected error on overflow")
	}
	v := r.ReadU32()
	if v != 0 {
		t.Errorf("expected 0 after error, got %d", v)
	}
}

func TestInitHandshake(t *testing.T) {
	w := protocol.NewWriter()
	w.WriteU32(3)
	w.WriteString("Go")
	params := w.Bytes()

	var buf bytes.Buffer
	err := protocol.WriteRequest(&buf, protocol.CmdInitialize, params)
	if err != nil {
		t.Fatalf("write request: %v", err)
	}

	data := buf.Bytes()

	payloadLen := binary.LittleEndian.Uint32(data[0:4])
	cmdID := binary.LittleEndian.Uint32(data[4:8])

	expectedPayloadLen := uint32(4 + len(params))
	if payloadLen != expectedPayloadLen {
		t.Errorf("payload len: expected %d, got %d", expectedPayloadLen, payloadLen)
	}
	if cmdID != protocol.CmdInitialize {
		t.Errorf("cmd: expected %d, got %d", protocol.CmdInitialize, cmdID)
	}

	paramReader := protocol.NewReader(data[8:])
	version := paramReader.ReadU32()
	clientName := paramReader.ReadString()

	if version != 3 {
		t.Errorf("version: expected 3, got %d", version)
	}
	if clientName != "Go" {
		t.Errorf("client name: expected 'Go', got '%s'", clientName)
	}
}

func TestResponseParsing(t *testing.T) {
	var buf bytes.Buffer

	body := make([]byte, 12)
	binary.LittleEndian.PutUint32(body[0:4], protocol.CmdInitialize)
	binary.LittleEndian.PutUint32(body[4:8], 1)
	binary.LittleEndian.PutUint32(body[8:12], 5684)

	header := make([]byte, 4)
	binary.LittleEndian.PutUint32(header, uint32(len(body)))
	buf.Write(header)
	buf.Write(body)

	resp, err := protocol.ReadResponse(&buf)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	if resp.CmdID != protocol.CmdInitialize {
		t.Errorf("cmd: expected %d, got %d", protocol.CmdInitialize, resp.CmdID)
	}
	if !resp.Success {
		t.Error("expected success")
	}
	if len(resp.Data) != 4 {
		t.Fatalf("expected 4 bytes data, got %d", len(resp.Data))
	}
	build := binary.LittleEndian.Uint32(resp.Data)
	if build != 5684 {
		t.Errorf("build: expected 5684, got %d", build)
	}
}
