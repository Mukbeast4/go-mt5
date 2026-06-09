package protocol_test

import (
	"testing"

	"github.com/mukbeast4/go-mt5/internal/protocol"
)

func fixedSlot(s string, slotBytes int) []byte {
	w := protocol.NewWriter()
	w.WriteFixedString(s, slotBytes)
	return w.Bytes()
}

func benchReadFixedString(b *testing.B, slot []byte, slotBytes int) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r := protocol.NewReader(slot)
		_ = r.ReadFixedString(slotBytes)
	}
}

func BenchmarkReadFixedString_Empty512(b *testing.B) {
	benchReadFixedString(b, make([]byte, 512), 512)
}

func BenchmarkReadFixedString_ASCII64(b *testing.B) {
	benchReadFixedString(b, fixedSlot("EURUSD", 64), 64)
}

func BenchmarkReadFixedString_NonASCII64(b *testing.B) {
	benchReadFixedString(b, fixedSlot("Euro é 中 франк", 64), 64)
}
