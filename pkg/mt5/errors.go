package mt5

import "fmt"

const (
	ErrCodeOK                  = 10009
	ErrCodeDone                = 10008
	ErrCodeRequote             = 10004
	ErrCodeReject              = 10006
	ErrCodeCancel              = 10007
	ErrCodeInvalidFill         = 10030
	ErrCodeConnection          = 10031
	ErrCodeTimeout             = 10012
	ErrCodeInvalidParams       = 10013
	ErrCodeInvalidOrder        = 10014
	ErrCodeInvalidVolume       = 10015
	ErrCodeInvalidPrice        = 10016
	ErrCodeInvalidStops        = 10017
	ErrCodeTradeDisabled       = 10018
	ErrCodeMarketClosed        = 10019
	ErrCodeInsufficientFunds   = 10020
	ErrCodePriceChanged        = 10021
	ErrCodeNoQuotes            = 10024
	ErrCodeOrderLocked         = 10025
	ErrCodeLongOnly            = 10027
	ErrCodeTooManyOrders       = 10028
	ErrCodeCloseOrderExist     = 10029
	ErrCodeAutoTradingDisabled = 10032
	ErrCodePositionClosed      = 10033
	ErrCodeLimitVolume         = 10034
)

type MT5Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *MT5Error) Error() string {
	return fmt.Sprintf("mt5 error %d: %s", e.Code, e.Message)
}

var ErrNotConnected = fmt.Errorf("mt5: not connected")
var ErrTimeout = fmt.Errorf("mt5: request timeout")
