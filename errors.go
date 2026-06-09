package gomt5

import (
	"errors"
	"fmt"
)

const (
	ResOK                   = 1
	ResEFail                = -1
	ResEInvalidParams       = -2
	ResENoMemory            = -3
	ResENotFound            = -4
	ResEInvalidVersion      = -5
	ResEAuthFailed          = -6
	ResEUnsupported         = -7
	ResEAutoTradingDisabled = -8
	ResEInternalFail        = -10000
	ResEInternalFailSend    = -10001
	ResEInternalFailReceive = -10002
	ResEInternalFailInit    = -10003
	ResEInternalFailConnect = -10004
	ResEInternalFailTimeout = -10005
)

const (
	RetcodeOK            uint32 = 10009
	RetcodeDone          uint32 = 10008
	RetcodeRequote       uint32 = 10004
	RetcodeReject        uint32 = 10006
	RetcodeCancel        uint32 = 10007
	RetcodeInvalidFill   uint32 = 10030
	RetcodeInvalidVolume uint32 = 10015
	RetcodeInvalidPrice  uint32 = 10016
	RetcodeInvalidStops  uint32 = 10017
	RetcodeTradeDisabled uint32 = 10018
	RetcodeMarketClosed  uint32 = 10019
	RetcodeNoQuotes      uint32 = 10024
	RetcodeTooManyOrders uint32 = 10028
)

type MT5Error struct {
	Code    int
	Message string
}

func (e *MT5Error) Error() string {
	return fmt.Sprintf("mt5 error %d: %s", e.Code, e.Message)
}

type TradeError struct {
	Result  TradeResult
	Request TradeRequest
}

func (e *TradeError) Error() string {
	return fmt.Sprintf("mt5 trade error %d: %s (action=%s symbol=%s)",
		e.Result.Retcode, e.Result.Comment, e.Request.Action, e.Request.Symbol)
}

var (
	ErrNotConnected  = errors.New("mt5: not connected")
	ErrTimeout       = errors.New("mt5: request timeout")
	ErrWindows       = errors.New("mt5: native pipe requires Windows")
	ErrFailed        = errors.New("mt5: request failed")
	ErrDisconnected  = errors.New("mt5: disconnected")
	ErrReconnecting  = errors.New("mt5: reconnecting")
	ErrTerminalClose = errors.New("mt5: terminal closed")
	ErrTradeDisabled = errors.New("mt5: trading disabled")
	ErrNoTick        = errors.New("mt5: no tick available")
	ErrNoBars        = errors.New("mt5: no bars available")
	ErrStaleBar      = errors.New("mt5: stale bar")
)
