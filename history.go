package gomt5

import (
	"fmt"

	"github.com/mukbeast4/go-mt5/internal/protocol"
)

func (c *Client) HistoryOrdersTotal(dateFrom, dateTo int64) (int, error) {
	w := protocol.NewWriter()
	w.WriteI64(dateFrom)
	w.WriteI64(dateTo)

	resp, err := c.send(protocol.CmdHistoryOrdersTotal, w.Bytes())
	if err != nil {
		return 0, err
	}
	r := protocol.NewReader(resp.Data)
	total := r.ReadU32()
	if r.Err() != nil {
		return 0, fmt.Errorf("decode history orders total: %w", r.Err())
	}
	return int(total), nil
}

func (c *Client) HistoryOrdersGet(dateFrom, dateTo int64) ([]Order, error) {
	w := protocol.NewWriter()
	w.WriteI64(dateFrom)
	w.WriteI64(dateTo)

	data, err := c.SendRaw(141, w.Bytes())
	if err != nil {
		return nil, err
	}

	return decodeOrders(data)
}

func (c *Client) HistoryDealsTotal(dateFrom, dateTo int64) (int, error) {
	w := protocol.NewWriter()
	w.WriteI64(dateFrom)
	w.WriteI64(dateTo)

	resp, err := c.send(protocol.CmdHistoryDealsTotal, w.Bytes())
	if err != nil {
		return 0, err
	}
	r := protocol.NewReader(resp.Data)
	total := r.ReadU32()
	if r.Err() != nil {
		return 0, fmt.Errorf("decode history deals total: %w", r.Err())
	}
	return int(total), nil
}

func (c *Client) HistoryDealsGet(dateFrom, dateTo int64) ([]Deal, error) {
	w := protocol.NewWriter()
	w.WriteI64(dateFrom)
	w.WriteI64(dateTo)

	data, err := c.SendRaw(151, w.Bytes())
	if err != nil {
		return nil, err
	}

	return decodeDeals(data)
}

func decodeDeals(data []byte) ([]Deal, error) {
	r := protocol.NewReader(data)
	count := int(r.ReadU32())

	deals := make([]Deal, 0, count)
	for i := 0; i < count; i++ {
		deal := Deal{
			Ticket:     r.ReadI64(),
			Order:      r.ReadI64(),
			Time:       r.ReadI64(),
			TimeMsc:    r.ReadI64(),
			Type:       DealType(r.ReadI64()),
			Entry:      DealEntry(r.ReadI64()),
			Magic:      r.ReadI64(),
			PositionID: r.ReadI64(),
			Reason:     r.ReadI64(),
			Volume:     r.ReadF64(),
			Price:      r.ReadF64(),
			Commission: r.ReadF64(),
			Swap:       r.ReadF64(),
			Profit:     r.ReadF64(),
			Fee:        r.ReadF64(),
			Symbol:     r.ReadString(),
			Comment:    r.ReadString(),
			ExternalID: r.ReadString(),
		}
		if r.Err() != nil {
			return nil, fmt.Errorf("decode deal %d: %w", i, r.Err())
		}
		deals = append(deals, deal)
	}
	return deals, nil
}
