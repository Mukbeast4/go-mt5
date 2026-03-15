package gomt5

import (
	"context"
	"fmt"

	"github.com/mukbeast4/go-mt5/internal/protocol"
)

func (c *Client) MarketBookAdd(ctx context.Context, symbol string) error {
	w := protocol.NewWriter()
	w.WriteString(symbol)

	_, err := c.SendRaw(ctx, protocol.CmdMarketBookAdd, w.Bytes())
	return err
}

func (c *Client) MarketBookGet(ctx context.Context, symbol string) ([]BookEntry, error) {
	w := protocol.NewWriter()
	w.WriteString(symbol)

	data, err := c.SendRaw(ctx, protocol.CmdMarketBookGet, w.Bytes())
	if err != nil {
		return nil, err
	}

	r := protocol.NewReader(data)
	count := int(r.ReadU32())

	entries := make([]BookEntry, 0, count)
	for i := 0; i < count; i++ {
		entry := BookEntry{
			Type:       BookType(r.ReadI64()),
			Price:      r.ReadF64(),
			Volume:     r.ReadI64(),
			VolumeReal: r.ReadF64(),
		}
		if r.Err() != nil {
			return nil, fmt.Errorf("decode book entry %d: %w", i, r.Err())
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func (c *Client) MarketBookRelease(ctx context.Context, symbol string) error {
	w := protocol.NewWriter()
	w.WriteString(symbol)

	_, err := c.SendRaw(ctx, protocol.CmdMarketBookRelease, w.Bytes())
	return err
}
