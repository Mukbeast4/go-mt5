package gomt5

import (
	"context"
	"fmt"

	"github.com/mukbeast4/go-mt5/internal/protocol"
)

func (c *Client) PositionsTotal(ctx context.Context) (int, error) {
	resp, err := c.send(ctx, protocol.CmdPositionsTotal, nil)
	if err != nil {
		return 0, err
	}
	r := protocol.NewReader(resp.Data)
	total := r.ReadU32()
	if r.Err() != nil {
		return 0, fmt.Errorf("decode positions total: %w", r.Err())
	}
	return int(total), nil
}

func (c *Client) PositionsGet(ctx context.Context, filter *PositionFilter) ([]Position, error) {
	var cmdID uint32
	w := protocol.NewWriter()

	switch {
	case filter == nil:
		cmdID = protocol.CmdPositionsGet
	case filter.Ticket != 0:
		cmdID = protocol.CmdPositionsGetByTicket
		w.WriteI64(filter.Ticket)
	case filter.Symbol != "":
		cmdID = protocol.CmdPositionsGetBySymbol
		w.WriteString(filter.Symbol)
	default:
		cmdID = protocol.CmdPositionsGet
	}

	data, err := c.SendRaw(ctx, cmdID, w.Bytes())
	if err != nil {
		return nil, err
	}

	positions, err := decodePositions(data)
	if err != nil {
		return nil, err
	}

	if filter != nil && filter.Group != "" {
		positions = filterByGroup(positions, filter.Group, func(p Position) string { return p.Symbol })
	}

	return positions, nil
}

func decodePositions(data []byte) ([]Position, error) {
	r := protocol.NewReader(data)
	count := int(r.ReadU32())

	positions := make([]Position, 0, count)
	for i := 0; i < count; i++ {
		pos := Position{
			Ticket:        r.ReadI64(),
			Time:          r.ReadI64(),
			TimeMsc:       r.ReadI64(),
			TimeUpdate:    r.ReadI64(),
			TimeUpdateMsc: r.ReadI64(),
			Type:          PositionType(r.ReadU32()),
			Magic:         r.ReadI64(),
			Identifier:    r.ReadI64(),
			Reason:        int64(r.ReadU32()),
			Volume:        r.ReadF64(),
			PriceOpen:     r.ReadF64(),
			PriceSL:       r.ReadF64(),
			PriceTP:       r.ReadF64(),
			PriceCurrent:  r.ReadF64(),
			Commission:    r.ReadF64(),
			Swap:          r.ReadF64(),
			Profit:        r.ReadF64(),
			Symbol:        r.ReadFixedString(64),
			Comment:       r.ReadFixedString(64),
			ExternalID:    r.ReadFixedString(64),
		}
		if r.Err() != nil {
			return nil, fmt.Errorf("decode position %d: %w", i, r.Err())
		}
		positions = append(positions, pos)
	}
	return positions, nil
}
