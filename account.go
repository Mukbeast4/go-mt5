package gomt5

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"log"
	"math"

	"github.com/mukbeast4/go-mt5/internal/protocol"
)

const (
	accountInfoNameSlotBytes     = 256
	accountInfoServerSlotBytes   = 128
	accountInfoCurrencySlotBytes = 64
	accountInfoCompanySlotBytes  = 256

	accountInfoStringsTotalBytes = accountInfoNameSlotBytes +
		accountInfoServerSlotBytes +
		accountInfoCurrencySlotBytes +
		accountInfoCompanySlotBytes

	accountInfoMiddleBytes = 139
)

var accountInfoDebugOnce bool

func (c *Client) Login(ctx context.Context, login int64, password, server string) error {
	w := protocol.NewWriter()
	w.WriteI64(login)
	w.WriteString(password)
	w.WriteString(server)

	_, err := c.SendRaw(ctx, protocol.CmdLogin, w.Bytes())
	return err
}

func (c *Client) AccountInfo(ctx context.Context) (*AccountInfo, error) {
	resp, err := c.send(ctx, protocol.CmdAccountInfo, nil)
	if err != nil {
		return nil, err
	}

	if len(resp.Data) < 8+accountInfoStringsTotalBytes {
		return nil, fmt.Errorf("decode account info: response too short (%d bytes)", len(resp.Data))
	}

	stringsOffset := len(resp.Data) - accountInfoStringsTotalBytes
	middle := resp.Data[8:stringsOffset]

	if !accountInfoDebugOnce {
		accountInfoDebugOnce = true
		log.Printf("[gomt5] AccountInfo debug: total=%d middle=%d strings_offset=%d",
			len(resp.Data), len(middle), stringsOffset)
		log.Printf("[gomt5] AccountInfo middle hex: %s", hex.EncodeToString(middle))
	}

	if len(middle) < accountInfoMiddleBytes {
		return nil, fmt.Errorf("decode account info: middle too short (%d bytes, want at least %d)", len(middle), accountInfoMiddleBytes)
	}

	readF64 := func(off int) float64 {
		return math.Float64frombits(binary.LittleEndian.Uint64(middle[off : off+8]))
	}
	readI32 := func(off int) int64 {
		return int64(int32(binary.LittleEndian.Uint32(middle[off : off+4])))
	}

	info := &AccountInfo{
		Login:             int64(binary.LittleEndian.Uint64(resp.Data[0:8])),
		TradeMode:         readI32(0),
		Leverage:          readI32(4),
		LimitOrders:       readI32(8),
		MarginSOMode:      readI32(12),
		TradeAllowed:      middle[16] != 0,
		TradeExpert:       middle[17] != 0,
		MarginMode:        readI32(18),
		CurrencyDigits:    readI32(22),
		FIFOClose:         middle[26] != 0,
		Balance:           readF64(27),
		Credit:            readF64(35),
		Profit:            readF64(43),
		Equity:            readF64(51),
		Margin:            readF64(59),
		FreeMargin:        readF64(67),
		MarginLevel:       readF64(75),
		MarginSOCall:      readF64(83),
		MarginSOSO:        readF64(91),
		MarginInitial:     readF64(99),
		MarginMaintenance: readF64(107),
		Assets:            readF64(115),
		Liabilities:       readF64(123),
		CommissionBlocked: readF64(131),
	}

	sr := protocol.NewReader(resp.Data[stringsOffset:])
	info.Name = sr.ReadFixedString(accountInfoNameSlotBytes)
	info.Server = sr.ReadFixedString(accountInfoServerSlotBytes)
	info.Currency = sr.ReadFixedString(accountInfoCurrencySlotBytes)
	info.Company = sr.ReadFixedString(accountInfoCompanySlotBytes)

	if sr.Err() != nil {
		return nil, fmt.Errorf("decode account info strings: %w", sr.Err())
	}
	return info, nil
}

func (c *Client) TerminalInfo(ctx context.Context) (*TerminalInfo, error) {
	resp, err := c.send(ctx, protocol.CmdTerminalInfoFull, nil)
	if err != nil {
		return nil, err
	}

	r := protocol.NewReader(resp.Data)
	info := &TerminalInfo{
		CommunityAccount:     r.ReadBool(),
		CommunityConnection:  r.ReadBool(),
		Connected:            r.ReadBool(),
		DLLsAllowed:          r.ReadBool(),
		TradeAllowed:         r.ReadBool(),
		TradeAPIDisabled:     r.ReadBool(),
		EmailEnabled:         r.ReadBool(),
		FTPEnabled:           r.ReadBool(),
		NotificationsEnabled: r.ReadBool(),
		MQID:                 r.ReadBool(),
		Build:                r.ReadI64(),
		MaxBars:              r.ReadI64(),
		CodePage:             r.ReadI64(),
		PingLast:             r.ReadI64(),

		CommunityBalance: r.ReadF64(),
		Retransmission:   r.ReadF64(),

		Company:        r.ReadString(),
		Name:           r.ReadString(),
		Language:       r.ReadString(),
		Path:           r.ReadString(),
		DataPath:       r.ReadString(),
		CommonDataPath: r.ReadString(),
	}

	if r.Err() != nil {
		return nil, fmt.Errorf("decode terminal info: %w", r.Err())
	}
	return info, nil
}

func (c *Client) Version(ctx context.Context) (*VersionInfo, error) {
	resp, err := c.send(ctx, protocol.CmdTerminalInfo, nil)
	if err != nil {
		return nil, err
	}

	r := protocol.NewReader(resp.Data)
	info := &VersionInfo{
		Version:   int(r.ReadI64()),
		Build:     int(r.ReadI64()),
		BuildDate: r.ReadString(),
	}

	if r.Err() != nil {
		return nil, fmt.Errorf("decode version: %w", r.Err())
	}
	return info, nil
}
