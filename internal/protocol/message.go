package protocol

import (
	"encoding/binary"
	"fmt"
	"io"
)

const maxPayloadSize = 64 * 1024 * 1024

func WriteRequest(w io.Writer, cmdID uint32, params []byte) error {
	buf := make([]byte, 8+len(params))
	binary.LittleEndian.PutUint32(buf[:4], uint32(4+len(params)))
	binary.LittleEndian.PutUint32(buf[4:8], cmdID)
	copy(buf[8:], params)
	if _, err := w.Write(buf); err != nil {
		return fmt.Errorf("write request: %w", err)
	}
	return nil
}

type Response struct {
	CmdID   uint32
	Success bool
	Data    []byte
}

func ReadResponse(r io.Reader) (*Response, error) {
	lenBuf := make([]byte, 4)
	if _, err := io.ReadFull(r, lenBuf); err != nil {
		return nil, fmt.Errorf("read payload length: %w", err)
	}
	payloadLen := binary.LittleEndian.Uint32(lenBuf)

	if payloadLen > maxPayloadSize {
		return nil, fmt.Errorf("payload too large: %d bytes", payloadLen)
	}
	if payloadLen < 8 {
		return nil, fmt.Errorf("payload too small: %d bytes", payloadLen)
	}

	payload := make([]byte, payloadLen)
	if _, err := io.ReadFull(r, payload); err != nil {
		return nil, fmt.Errorf("read payload: %w", err)
	}

	cmdID := binary.LittleEndian.Uint32(payload[0:4])
	success := binary.LittleEndian.Uint32(payload[4:8])

	resp := &Response{
		CmdID:   cmdID,
		Success: success != 0,
	}

	if len(payload) > 8 {
		resp.Data = payload[8:]
	}

	return resp, nil
}
