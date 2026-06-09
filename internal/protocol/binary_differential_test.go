package protocol_test

import (
	"encoding/binary"
	"fmt"
	"io"
	"testing"
	"unicode/utf16"

	"github.com/mukbeast4/go-mt5/internal/protocol"
)

// refReader is a frozen copy of the pre-fast-path ReadFixedString. It is the
// differential-fuzz reference: do not modernize or optimize it.
type refReader struct {
	data []byte
	pos  int
	err  error
}

func (r *refReader) readFixedString(slotBytes int) string {
	if r.err != nil {
		return ""
	}
	if slotBytes%2 != 0 {
		r.err = fmt.Errorf("ReadFixedString: odd slot size %d", slotBytes)
		return ""
	}
	if r.pos+slotBytes > len(r.data) {
		r.err = io.ErrUnexpectedEOF
		return ""
	}
	end := r.pos + slotBytes
	buf := r.data[r.pos:end]

	u16 := make([]uint16, 0, slotBytes/2)
	for i := 0; i+1 < len(buf); i += 2 {
		c := binary.LittleEndian.Uint16(buf[i:])
		if c == 0 {
			break
		}
		u16 = append(u16, c)
	}
	r.pos = end
	return string(utf16.Decode(u16))
}

func unitsLE(units ...uint16) []byte {
	b := make([]byte, 0, len(units)*2)
	for _, u := range units {
		b = binary.LittleEndian.AppendUint16(b, u)
	}
	return b
}

func FuzzReadFixedString_Differential(f *testing.F) {
	f.Add([]byte{}, 0)
	f.Add(make([]byte, 64), 64)
	f.Add(unitsLE('E', 'U', 'R', 'U', 'S', 'D', 0, 0), 16)
	f.Add(unitsLE('E', 'U', 'R', 0, 'X', 'X'), 12)
	f.Add(unitsLE('A', 'B', 'C', 'D'), 8)
	f.Add(unitsLE(0x00E9, 0), 4)
	f.Add(unitsLE(0x4E2D, 0x6587, 0), 6)
	f.Add(unitsLE(0xD83D, 0xDE00, 0), 6)
	f.Add(unitsLE(0xD83D, 0), 4)
	f.Add(unitsLE(0xDE00, 0), 4)
	f.Add(unitsLE(0xD83D, 0, 0xDE00), 6)
	f.Add(make([]byte, 10), 3)
	f.Add(make([]byte, 10), 64)

	f.Fuzz(func(t *testing.T, data []byte, slotBytes int) {
		if slotBytes < 0 || slotBytes > 1<<20 {
			t.Skip()
		}
		nr := protocol.NewReader(data)
		rr := &refReader{data: data}

		got := nr.ReadFixedString(slotBytes)
		want := rr.readFixedString(slotBytes)
		if got != want {
			t.Fatalf("output diverged (slot=%d): got %q want %q", slotBytes, got, want)
		}
		if nr.Pos() != rr.pos {
			t.Fatalf("pos diverged (slot=%d): got %d want %d", slotBytes, nr.Pos(), rr.pos)
		}
		gotErr, wantErr := nr.Err(), rr.err
		if (gotErr == nil) != (wantErr == nil) {
			t.Fatalf("err presence diverged (slot=%d): got %v want %v", slotBytes, gotErr, wantErr)
		}
		if gotErr != nil && gotErr.Error() != wantErr.Error() {
			t.Fatalf("err message diverged (slot=%d): got %q want %q", slotBytes, gotErr, wantErr)
		}
	})
}
