package protocol

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"unicode/utf16"
)

type Writer struct {
	buf []byte
}

func NewWriter() *Writer {
	return &Writer{buf: make([]byte, 0, 128)}
}

func (w *Writer) WriteU8(v uint8) {
	w.buf = append(w.buf, v)
}

func (w *Writer) WriteBool(v bool) {
	if v {
		w.buf = append(w.buf, 1)
	} else {
		w.buf = append(w.buf, 0)
	}
}

func (w *Writer) WriteU32(v uint32) {
	w.buf = binary.LittleEndian.AppendUint32(w.buf, v)
}

func (w *Writer) WriteI32(v int32) {
	w.WriteU32(uint32(v))
}

func (w *Writer) WriteU64(v uint64) {
	w.buf = binary.LittleEndian.AppendUint64(w.buf, v)
}

func (w *Writer) WriteI64(v int64) {
	w.WriteU64(uint64(v))
}

func (w *Writer) WriteF64(v float64) {
	w.WriteU64(math.Float64bits(v))
}

func (w *Writer) WriteString(s string) {
	runes := utf16.Encode([]rune(s))
	w.buf = binary.LittleEndian.AppendUint32(w.buf, uint32(len(runes)))
	for _, r := range runes {
		w.buf = binary.LittleEndian.AppendUint16(w.buf, r)
	}
}

func (w *Writer) Bytes() []byte {
	return w.buf
}

func (w *Writer) Len() int {
	return len(w.buf)
}

type Reader struct {
	data []byte
	pos  int
	err  error
}

func NewReader(data []byte) *Reader {
	return &Reader{data: data}
}

func (r *Reader) ReadU32() uint32 {
	if r.err != nil {
		return 0
	}
	if r.pos+4 > len(r.data) {
		r.err = io.ErrUnexpectedEOF
		return 0
	}
	v := binary.LittleEndian.Uint32(r.data[r.pos:])
	r.pos += 4
	return v
}

func (r *Reader) ReadI32() int32 {
	return int32(r.ReadU32())
}

func (r *Reader) ReadU64() uint64 {
	if r.err != nil {
		return 0
	}
	if r.pos+8 > len(r.data) {
		r.err = io.ErrUnexpectedEOF
		return 0
	}
	v := binary.LittleEndian.Uint64(r.data[r.pos:])
	r.pos += 8
	return v
}

func (r *Reader) ReadI64() int64 {
	return int64(r.ReadU64())
}

func (r *Reader) ReadF64() float64 {
	return math.Float64frombits(r.ReadU64())
}

// ReadBool reads an LE64 boolean (used in most response payloads).
func (r *Reader) ReadBool() bool {
	return r.ReadI64() != 0
}

// ReadBool1 reads a 1-byte boolean. Used by struct-style payloads such as
// the SymbolInfo response (cmd 170 / 174), where booleans are packed as a
// single byte rather than LE64.
func (r *Reader) ReadBool1() bool {
	if r.err != nil {
		return false
	}
	if r.pos+1 > len(r.data) {
		r.err = io.ErrUnexpectedEOF
		return false
	}
	v := r.data[r.pos] != 0
	r.pos++
	return v
}

// ReadFixedString reads a fixed-width null-padded UTF-16LE string slot. The
// content starts at the first byte and is terminated by a UTF-16 NUL pair or
// the end of the slot, whichever comes first. The cursor always advances by
// exactly slotBytes regardless of the string length.
//
// Used by struct-style responses (e.g. SymbolInfo) where each string field
// occupies a fixed buffer matching the MT5 internal struct layout.
func (r *Reader) ReadFixedString(slotBytes int) string {
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

func (r *Reader) ReadString() string {
	charCount := r.ReadU32()
	if r.err != nil {
		return ""
	}
	byteCount := int(charCount) * 2
	if r.pos+byteCount > len(r.data) {
		r.err = io.ErrUnexpectedEOF
		return ""
	}
	u16 := make([]uint16, charCount)
	for i := 0; i < int(charCount); i++ {
		u16[i] = binary.LittleEndian.Uint16(r.data[r.pos:])
		r.pos += 2
	}
	return string(utf16.Decode(u16))
}

func (r *Reader) ReadBytes(n int) []byte {
	if r.err != nil {
		return nil
	}
	if r.pos+n > len(r.data) {
		r.err = io.ErrUnexpectedEOF
		return nil
	}
	b := make([]byte, n)
	copy(b, r.data[r.pos:r.pos+n])
	r.pos += n
	return b
}

func (r *Reader) Remaining() int {
	return len(r.data) - r.pos
}

func (r *Reader) RemainingBytes() []byte {
	if r.pos >= len(r.data) {
		return nil
	}
	return r.data[r.pos:]
}

func (r *Reader) Err() error {
	return r.err
}

func (r *Reader) Pos() int {
	return r.pos
}

func (r *Reader) Skip(n int) {
	if r.err != nil {
		return
	}
	if r.pos+n > len(r.data) {
		r.err = io.ErrUnexpectedEOF
		return
	}
	r.pos += n
}
