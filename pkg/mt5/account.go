package mt5

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mukbeast4/go-mt5/internal/protocol"
)

func (c *Client) AccountInfo(ctx context.Context) (*AccountInfo, error) {
	resp, err := c.send(ctx, protocol.ActionAccountInfo, nil)
	if err != nil {
		return nil, err
	}
	var info AccountInfo
	if err := json.Unmarshal(resp.Data, &info); err != nil {
		return nil, fmt.Errorf("decode account info: %w", err)
	}
	return &info, nil
}

func (c *Client) TerminalInfo(ctx context.Context) (*TerminalInfo, error) {
	resp, err := c.send(ctx, protocol.ActionTerminalInfo, nil)
	if err != nil {
		return nil, err
	}
	var info TerminalInfo
	if err := json.Unmarshal(resp.Data, &info); err != nil {
		return nil, fmt.Errorf("decode terminal info: %w", err)
	}
	return &info, nil
}
