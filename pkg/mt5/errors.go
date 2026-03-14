package mt5

import "fmt"

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

	RetcodeOK              = 10009
	RetcodeDone            = 10008
	RetcodeRequote         = 10004
	RetcodeReject          = 10006
	RetcodeCancel          = 10007
	RetcodeInvalidFill     = 10030
	RetcodeInvalidVolume   = 10015
	RetcodeInvalidPrice    = 10016
	RetcodeInvalidStops    = 10017
	RetcodeTradeDisabled   = 10018
	RetcodeMarketClosed    = 10019
	RetcodeNoQuotes        = 10024
	RetcodeTooManyOrders   = 10028
)

type MT5Error struct {
	Code    int
	Message string
}

func (e *MT5Error) Error() string {
	return fmt.Sprintf("mt5 error %d: %s", e.Code, e.Message)
}

var (
	ErrNotConnected = fmt.Errorf("mt5: not connected")
	ErrTimeout      = fmt.Errorf("mt5: request timeout")
	ErrWindows      = fmt.Errorf("mt5: native pipe requires Windows")
	ErrFailed       = fmt.Errorf("mt5: request failed")
)
