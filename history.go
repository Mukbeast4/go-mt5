package gomt5

import (
	"context"
	"fmt"

	"github.com/mukbeast4/go-mt5/internal/protocol"
)

func (c *Client) HistoryOrdersTotal(ctx context.Context, dateFrom, dateTo int64) (int, error) {
	w := protocol.NewWriter()
	w.WriteI64(dateFrom)
	w.WriteI64(dateTo)

	resp, err := c.send(ctx, protocol.CmdHistoryOrdersTotal, w.Bytes())
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

func (c *Client) HistoryOrdersGet(ctx context.Context, filter *HistoryFilter) ([]Order, error) {
	var cmdID uint32
	w := protocol.NewWriter()

	switch {
	case filter == nil:
		cmdID = protocol.CmdHistoryOrdersGet
		w.WriteI64(0)
		w.WriteI64(0)
	case filter.Ticket != 0:
		cmdID = protocol.CmdHistoryOrdersGetTkt
		w.WriteI64(filter.Ticket)
	case filter.Symbol != "":
		cmdID = protocol.CmdHistoryOrdersGetSym
		w.WriteI64(filter.DateFrom)
		w.WriteI64(filter.DateTo)
		w.WriteString(filter.Symbol)
	default:
		cmdID = protocol.CmdHistoryOrdersGet
		w.WriteI64(filter.DateFrom)
		w.WriteI64(filter.DateTo)
	}

	data, err := c.SendRaw(ctx, cmdID, w.Bytes())
	if err != nil {
		return nil, err
	}

	orders, err := decodeOrders(data)
	if err != nil {
		return nil, err
	}

	if filter != nil && filter.Group != "" {
		orders = filterByGroup(orders, filter.Group, func(o Order) string { return o.Symbol })
	}

	return orders, nil
}

func (c *Client) HistoryDealsTotal(ctx context.Context, dateFrom, dateTo int64) (int, error) {
	w := protocol.NewWriter()
	w.WriteI64(dateFrom)
	w.WriteI64(dateTo)

	resp, err := c.send(ctx, protocol.CmdHistoryDealsTotal, w.Bytes())
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

func (c *Client) HistoryDealsGet(ctx context.Context, filter *HistoryFilter) ([]Deal, error) {
	var cmdID uint32
	w := protocol.NewWriter()

	switch {
	case filter == nil:
		cmdID = protocol.CmdHistoryDealsGet
		w.WriteI64(0)
		w.WriteI64(0)
	case filter.Ticket != 0:
		cmdID = protocol.CmdHistoryDealsGetTkt
		w.WriteI64(filter.Ticket)
	case filter.Symbol != "":
		cmdID = protocol.CmdHistoryDealsGetSym
		w.WriteI64(filter.DateFrom)
		w.WriteI64(filter.DateTo)
		w.WriteString(filter.Symbol)
	default:
		cmdID = protocol.CmdHistoryDealsGet
		w.WriteI64(filter.DateFrom)
		w.WriteI64(filter.DateTo)
	}

	data, err := c.SendRaw(ctx, cmdID, w.Bytes())
	if err != nil {
		return nil, err
	}

	deals, err := decodeDeals(data)
	if err != nil {
		return nil, err
	}

	if filter != nil && filter.Group != "" {
		deals = filterByGroup(deals, filter.Group, func(d Deal) string { return d.Symbol })
	}

	return deals, nil
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
			Type:       DealType(r.ReadU32()),
			Entry:      DealEntry(r.ReadU32()),
			Magic:      r.ReadI64(),
			PositionID: r.ReadI64(),
			Reason:     int64(r.ReadU32()),
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
