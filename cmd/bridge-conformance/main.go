// bridge-conformance verifies the TCP framing and minimal protobuf exchange
// without relying on generated bindings. It is intentionally small so it can
// be run alongside the retained Go MT5 reference during a Rust bridge cutover.
package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"time"
)

const (
	protocolVersion uint16 = 1
	frameHeader            = 20 // bytes after frame_length before metadata
	maxFrameLength         = 1024 * 1024

	messageHello    uint16 = 1
	messageHelloAck uint16 = 2
	messagePing     uint16 = 11
	messagePong     uint16 = 12
)

func main() {
	address := envOr("MT5_BRIDGE_ADDR", "127.0.0.1:19550")
	token := os.Getenv("MT5_BRIDGE_TOKEN")
	if token == "" {
		fatal(errors.New("MT5_BRIDGE_TOKEN is required"))
	}

	connection, err := net.DialTimeout("tcp", address, 5*time.Second)
	if err != nil {
		fatal(err)
	}
	defer connection.Close()
	if err := connection.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		fatal(err)
	}

	// Hello: client_id (field 1) and token (field 2), both length-delimited.
	hello := appendLengthDelimited(nil, 1, []byte("go-bridge-conformance"))
	hello = appendLengthDelimited(hello, 2, []byte(token))
	if err := writeFrame(connection, messageHello, 0, hello, nil); err != nil {
		fatal(err)
	}
	ack, err := readFrame(connection)
	if err != nil {
		fatal(err)
	}
	if ack.messageType != messageHelloAck || ack.requestID != 0 {
		fatal(fmt.Errorf("expected HelloAck(0), received type=%d id=%d", ack.messageType, ack.requestID))
	}

	// Ping carries a protobuf uint64 nonce as field 1. The response proves both
	// TCP framing and raw protobuf compatibility between Go and Rust.
	const nonce uint64 = 0x5A17_2026
	ping := appendVarintField(nil, 1, nonce)
	if err := writeFrame(connection, messagePing, 0, ping, nil); err != nil {
		fatal(err)
	}
	pong, err := readFrame(connection)
	if err != nil {
		fatal(err)
	}
	if pong.messageType != messagePong || pong.requestID != 0 {
		fatal(fmt.Errorf("expected Pong(0), received type=%d id=%d", pong.messageType, pong.requestID))
	}
	receivedNonce, err := readSingleVarintField(pong.metadata, 1)
	if err != nil || receivedNonce != nonce {
		fatal(fmt.Errorf("pong nonce mismatch: got=%d err=%v", receivedNonce, err))
	}

	fmt.Printf("bridge TCP conformance passed: hello ack %d bytes, ping nonce %d\n", len(ack.metadata), nonce)
}

type frame struct {
	messageType uint16
	requestID   uint64
	metadata    []byte
	payload     []byte
}

func writeFrame(w io.Writer, messageType uint16, requestID uint64, metadata, payload []byte) error {
	if len(metadata) > 64*1024 {
		return errors.New("metadata exceeds v1 maximum")
	}
	length := frameHeader + len(metadata) + len(payload)
	if length > maxFrameLength {
		return errors.New("frame exceeds v1 maximum")
	}
	buffer := make([]byte, 4+length)
	binary.LittleEndian.PutUint32(buffer[0:4], uint32(length))
	binary.LittleEndian.PutUint16(buffer[4:6], protocolVersion)
	binary.LittleEndian.PutUint16(buffer[6:8], messageType)
	// flags [8:12] remain zero in protocol v1.
	binary.LittleEndian.PutUint64(buffer[12:20], requestID)
	binary.LittleEndian.PutUint32(buffer[20:24], uint32(len(metadata)))
	copy(buffer[24:], metadata)
	copy(buffer[24+len(metadata):], payload)
	for len(buffer) > 0 {
		written, err := w.Write(buffer)
		if err != nil {
			return err
		}
		if written == 0 {
			return io.ErrShortWrite
		}
		buffer = buffer[written:]
	}
	return nil
}

func readFrame(r io.Reader) (frame, error) {
	var lengthBytes [4]byte
	if _, err := io.ReadFull(r, lengthBytes[:]); err != nil {
		return frame{}, err
	}
	length := int(binary.LittleEndian.Uint32(lengthBytes[:]))
	if length < frameHeader || length > maxFrameLength {
		return frame{}, fmt.Errorf("invalid frame length %d", length)
	}
	body := make([]byte, length)
	if _, err := io.ReadFull(r, body); err != nil {
		return frame{}, err
	}
	if binary.LittleEndian.Uint16(body[0:2]) != protocolVersion {
		return frame{}, errors.New("unsupported bridge protocol version")
	}
	if binary.LittleEndian.Uint32(body[4:8]) != 0 {
		return frame{}, errors.New("v1 frame contains nonzero flags")
	}
	metadataLength := int(binary.LittleEndian.Uint32(body[16:20]))
	payloadLength := length - frameHeader
	if metadataLength < 0 || metadataLength > 64*1024 || metadataLength > payloadLength {
		return frame{}, errors.New("invalid metadata length")
	}
	return frame{
		messageType: binary.LittleEndian.Uint16(body[2:4]),
		requestID:   binary.LittleEndian.Uint64(body[8:16]),
		metadata:    body[20 : 20+metadataLength],
		payload:     body[20+metadataLength:],
	}, nil
}

func appendLengthDelimited(dst []byte, field uint64, value []byte) []byte {
	dst = appendVarint(dst, field<<3|2)
	dst = appendVarint(dst, uint64(len(value)))
	return append(dst, value...)
}

func appendVarintField(dst []byte, field, value uint64) []byte {
	return appendVarint(appendVarint(dst, field<<3), value)
}

func appendVarint(dst []byte, value uint64) []byte {
	for value >= 0x80 {
		dst = append(dst, byte(value)|0x80)
		value >>= 7
	}
	return append(dst, byte(value))
}

func readSingleVarintField(data []byte, wantedField uint64) (uint64, error) {
	for len(data) > 0 {
		key, consumed := readVarint(data)
		if consumed == 0 {
			return 0, errors.New("invalid protobuf field key")
		}
		data = data[consumed:]
		if key>>3 != wantedField || key&7 != 0 {
			return 0, errors.New("unexpected protobuf field")
		}
		value, consumed := readVarint(data)
		if consumed == 0 || consumed != len(data) {
			return 0, errors.New("invalid protobuf varint")
		}
		return value, nil
	}
	return 0, errors.New("missing protobuf field")
}

func readVarint(data []byte) (uint64, int) {
	var value uint64
	for index, octet := range data {
		if index == 10 || index == 9 && octet > 1 {
			return 0, 0
		}
		value |= uint64(octet&0x7f) << (7 * index)
		if octet&0x80 == 0 {
			return value, index + 1
		}
	}
	return 0, 0
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "bridge conformance:", err)
	os.Exit(1)
}
