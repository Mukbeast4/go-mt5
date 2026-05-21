package protocol_test

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/mukbeast4/go-mt5/internal/protocol"
)

func FuzzReader_ReadString(f *testing.F) {
	f.Add([]byte{0x00, 0x00, 0x00, 0x00})
	f.Add([]byte{0x06, 0x00, 0x00, 0x00, 'P', 0, 'y', 0, 't', 0, 'h', 0, 'o', 0, 'n', 0})
	f.Add([]byte{0x01, 0x00, 0x00, 0x80})
	f.Add([]byte{0xFF, 0xFF, 0xFF, 0xFF})
	f.Add([]byte{0x10, 0x00, 0x00, 0x00})
	f.Add([]byte{})
	f.Add([]byte{0x02, 0x00, 0x00, 0x00, 0xFF, 0xFF, 0xFF, 0xFF})

	f.Fuzz(func(t *testing.T, data []byte) {
		r := protocol.NewReader(data)
		_ = r.ReadString()
		if r.Err() == nil && r.Pos() > len(data) {
			t.Errorf("cursor advanced past end: pos=%d, len(data)=%d", r.Pos(), len(data))
		}
	})
}

func FuzzReader_ReadFixedString(f *testing.F) {
	f.Add([]byte{}, 0)
	f.Add(make([]byte, 64), 64)
	f.Add(make([]byte, 256), 64)
	f.Add(make([]byte, 32), 64)
	f.Add(make([]byte, 100), 100)
	f.Add(make([]byte, 100), 99)

	f.Fuzz(func(t *testing.T, data []byte, slotBytes int) {
		if slotBytes < 0 || slotBytes > 1<<20 {
			t.Skip()
		}
		r := protocol.NewReader(data)
		_ = r.ReadFixedString(slotBytes)
	})
}

func FuzzReadResponse(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{0x00, 0x00, 0x00, 0x00})
	f.Add([]byte{0x08, 0x00, 0x00, 0x00, 0x04, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00})
	f.Add([]byte{0xFF, 0xFF, 0xFF, 0xFF})

	var oversize [12]byte
	binary.LittleEndian.PutUint32(oversize[:4], 64*1024*1024+1)
	f.Add(oversize[:])

	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = protocol.ReadResponse(bytes.NewReader(data))
	})
}
