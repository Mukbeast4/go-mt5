package mt5

import (
	"fmt"

	"github.com/mukbeast4/go-mt5/internal/protocol"
)

func (c *Client) PositionsTotal() (int, error) {
	resp, err := c.send(protocol.CmdPositionsTotal, nil)
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

func (c *Client) PositionsGet(symbol string) ([]Position, error) {
	w := protocol.NewWriter()
	w.WriteString(symbol)

	data, err := c.SendRaw(121, w.Bytes())
	if err != nil {
		return nil, err
	}

	return decodePositions(data)
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
			Type:          PositionType(r.ReadI64()),
			Magic:         r.ReadI64(),
			Identifier:    r.ReadI64(),
			Reason:        r.ReadI64(),
			Volume:        r.ReadF64(),
			PriceOpen:     r.ReadF64(),
			PriceCurrent:  r.ReadF64(),
			PriceSL:       r.ReadF64(),
			PriceTP:       r.ReadF64(),
			Swap:          r.ReadF64(),
			Profit:        r.ReadF64(),
			Symbol:        r.ReadString(),
			Comment:       r.ReadString(),
			ExternalID:    r.ReadString(),
		}
		if r.Err() != nil {
			return nil, fmt.Errorf("decode position %d: %w", i, r.Err())
		}
		positions = append(positions, pos)
	}
	return positions, nil
}
