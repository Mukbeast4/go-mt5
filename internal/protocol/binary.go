package protocol

import (
	"encoding/binary"
	"io"
	"math"
	"unicode/utf16"
)

type Writer struct {
	buf []byte
}

func NewWriter() *Writer {
	return &Writer{}
}

func (w *Writer) WriteU32(v uint32) {
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, v)
	w.buf = append(w.buf, b...)
}

func (w *Writer) WriteI32(v int32) {
	w.WriteU32(uint32(v))
}

func (w *Writer) WriteU64(v uint64) {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, v)
	w.buf = append(w.buf, b...)
}

func (w *Writer) WriteI64(v int64) {
	w.WriteU64(uint64(v))
}

func (w *Writer) WriteF64(v float64) {
	w.WriteU64(math.Float64bits(v))
}

func (w *Writer) WriteString(s string) {
	runes := utf16.Encode([]rune(s))
	w.WriteU32(uint32(len(runes)))
	for _, r := range runes {
		b := make([]byte, 2)
		binary.LittleEndian.PutUint16(b, r)
		w.buf = append(w.buf, b...)
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

func (r *Reader) ReadBool() bool {
	return r.ReadI64() != 0
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
