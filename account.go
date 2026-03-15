package gomt5

import (
	"context"
	"fmt"

	"github.com/mukbeast4/go-mt5/internal/protocol"
)

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

	r := protocol.NewReader(resp.Data)
	info := &AccountInfo{
		Login:          r.ReadI64(),
		TradeMode:      r.ReadI64(),
		Leverage:       r.ReadI64(),
		LimitOrders:    r.ReadI64(),
		MarginSOMode:   r.ReadI64(),
		TradeAllowed:   r.ReadBool(),
		TradeExpert:    r.ReadBool(),
		MarginMode:     r.ReadI64(),
		CurrencyDigits: r.ReadI64(),
		FIFOClose:      r.ReadBool(),

		Balance:           r.ReadF64(),
		Credit:            r.ReadF64(),
		Profit:            r.ReadF64(),
		Equity:            r.ReadF64(),
		Margin:            r.ReadF64(),
		FreeMargin:        r.ReadF64(),
		MarginLevel:       r.ReadF64(),
		MarginSOCall:      r.ReadF64(),
		MarginSOSO:        r.ReadF64(),
		MarginInitial:     r.ReadF64(),
		MarginMaintenance: r.ReadF64(),
		Assets:            r.ReadF64(),
		Liabilities:       r.ReadF64(),
		CommissionBlocked: r.ReadF64(),

		Name:     r.ReadString(),
		Server:   r.ReadString(),
		Currency: r.ReadString(),
		Company:  r.ReadString(),
	}

	if r.Err() != nil {
		return nil, fmt.Errorf("decode account info: %w", r.Err())
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
