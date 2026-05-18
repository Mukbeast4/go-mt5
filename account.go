package gomt5

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"log"

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

	info := &AccountInfo{
		Login: int64(binary.LittleEndian.Uint64(resp.Data[0:8])),
	}

	mr := protocol.NewReader(middle)
	info.TradeMode = mr.ReadI64()
	info.Leverage = mr.ReadI64()
	info.LimitOrders = mr.ReadI64()
	info.MarginSOMode = mr.ReadI64()
	info.TradeAllowed = mr.ReadBool()
	info.TradeExpert = mr.ReadBool()
	info.MarginMode = mr.ReadI64()
	info.CurrencyDigits = mr.ReadI64()
	info.FIFOClose = mr.ReadBool()
	info.Balance = mr.ReadF64()
	info.Credit = mr.ReadF64()
	info.Profit = mr.ReadF64()
	info.Equity = mr.ReadF64()
	info.Margin = mr.ReadF64()
	info.FreeMargin = mr.ReadF64()
	info.MarginLevel = mr.ReadF64()
	info.MarginSOCall = mr.ReadF64()
	info.MarginSOSO = mr.ReadF64()
	info.MarginInitial = mr.ReadF64()
	info.MarginMaintenance = mr.ReadF64()
	info.Assets = mr.ReadF64()
	info.Liabilities = mr.ReadF64()
	info.CommissionBlocked = mr.ReadF64()

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
