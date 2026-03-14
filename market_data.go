package gomt5

import (
	"fmt"

	"github.com/mukbeast4/go-mt5/internal/protocol"
)

func (c *Client) CopyRatesFromPos(symbol string, timeframe Timeframe, startPos, count int) ([]Rate, error) {
	w := protocol.NewWriter()
	w.WriteString(symbol)
	w.WriteU32(uint32(timeframe))
	w.WriteU32(uint32(startPos))
	w.WriteU32(uint32(count))

	resp, err := c.send(protocol.CmdCopyRatesFromPos, w.Bytes())
	if err != nil {
		return nil, err
	}

	return decodeRates(resp.Data)
}

func (c *Client) CopyRatesFrom(symbol string, timeframe Timeframe, dateFrom int64, count int) ([]Rate, error) {
	w := protocol.NewWriter()
	w.WriteString(symbol)
	w.WriteU32(uint32(timeframe))
	w.WriteI64(dateFrom)
	w.WriteU32(uint32(count))

	data, err := c.SendRaw(108, w.Bytes())
	if err != nil {
		return nil, err
	}

	return decodeRates(data)
}

func (c *Client) CopyRatesRange(symbol string, timeframe Timeframe, dateFrom, dateTo int64) ([]Rate, error) {
	w := protocol.NewWriter()
	w.WriteString(symbol)
	w.WriteU32(uint32(timeframe))
	w.WriteI64(dateFrom)
	w.WriteI64(dateTo)

	data, err := c.SendRaw(109, w.Bytes())
	if err != nil {
		return nil, err
	}

	return decodeRates(data)
}

func (c *Client) CopyTicksFrom(symbol string, dateFrom int64, count int, flags CopyTicksFlag) ([]Tick, error) {
	w := protocol.NewWriter()
	w.WriteString(symbol)
	w.WriteI64(dateFrom)
	w.WriteU32(uint32(count))
	w.WriteU32(uint32(flags))

	resp, err := c.send(protocol.CmdCopyTicksFrom, w.Bytes())
	if err != nil {
		return nil, err
	}

	return decodeTicks(resp.Data)
}

func (c *Client) CopyTicksRange(symbol string, dateFrom, dateTo int64, flags CopyTicksFlag) ([]Tick, error) {
	w := protocol.NewWriter()
	w.WriteString(symbol)
	w.WriteI64(dateFrom)
	w.WriteI64(dateTo)
	w.WriteU32(uint32(flags))

	data, err := c.SendRaw(105, w.Bytes())
	if err != nil {
		return nil, err
	}

	return decodeTicks(data)
}

func decodeRates(data []byte) ([]Rate, error) {
	r := protocol.NewReader(data)
	count := int(r.ReadU32())

	rates := make([]Rate, 0, count)
	for i := 0; i < count; i++ {
		rate := Rate{
			Time:       r.ReadI64(),
			Open:       r.ReadF64(),
			High:       r.ReadF64(),
			Low:        r.ReadF64(),
			Close:      r.ReadF64(),
			TickVolume: r.ReadI64(),
			Spread:     r.ReadI32(),
			RealVolume: r.ReadI64(),
		}
		if r.Err() != nil {
			return nil, fmt.Errorf("decode rate %d: %w", i, r.Err())
		}
		rates = append(rates, rate)
	}
	return rates, nil
}

func decodeTicks(data []byte) ([]Tick, error) {
	r := protocol.NewReader(data)
	count := int(r.ReadU32())

	ticks := make([]Tick, 0, count)
	for i := 0; i < count; i++ {
		tick := Tick{
			Time:    r.ReadI64(),
			Bid:     r.ReadF64(),
			Ask:     r.ReadF64(),
			Last:    r.ReadF64(),
			Volume:  r.ReadU64(),
			TimeMsc: r.ReadI64(),
			Flags:   r.ReadU32(),
		}
		if r.Err() != nil {
			return nil, fmt.Errorf("decode tick %d: %w", i, r.Err())
		}
		ticks = append(ticks, tick)
	}
	return ticks, nil
}
