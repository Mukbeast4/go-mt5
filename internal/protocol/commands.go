package protocol

const (
	CmdTerminalInfo     uint32 = 1
	CmdInitialize       uint32 = 4
	CmdSetAPIVersion    uint32 = 109
	CmdTerminalInfoFull uint32 = 180
)

const (
	CmdLogin       uint32 = 100
	CmdAccountInfo uint32 = 190
)

const (
	CmdSymbolInfo        uint32 = 170
	CmdSymbolSelect      uint32 = 171
	CmdSymbolInfoTick    uint32 = 172
	CmdSymbolsTotal      uint32 = 173
	CmdSymbolsGet        uint32 = 174
	CmdSymbolsGetByGroup uint32 = 175
)

const (
	CmdCopyTicksFrom    uint32 = 104
	CmdCopyTicksRange   uint32 = 105
	CmdCopyRatesFrom    uint32 = 106
	CmdCopyRatesRange   uint32 = 107
	CmdCopyRatesFromPos uint32 = 108
)

const (
	CmdPositionsTotal       uint32 = 120
	CmdPositionsGet         uint32 = 121
	CmdPositionsGetBySymbol uint32 = 122
	CmdPositionsGetByTicket uint32 = 123
)

const (
	CmdOrdersTotal       uint32 = 130
	CmdOrdersGet         uint32 = 131
	CmdOrdersGetBySymbol uint32 = 132
	CmdOrdersGetByTicket uint32 = 133
)

const (
	CmdHistoryOrdersTotal  uint32 = 140
	CmdHistoryOrdersGet    uint32 = 141
	CmdHistoryOrdersGetSym uint32 = 142
	CmdHistoryOrdersGetTkt uint32 = 143
)

const (
	CmdHistoryDealsTotal  uint32 = 150
	CmdHistoryDealsGet    uint32 = 151
	CmdHistoryDealsGetSym uint32 = 152
	CmdHistoryDealsGetTkt uint32 = 153
)

const (
	CmdOrderCheck      uint32 = 160
	CmdOrderSend       uint32 = 161
	CmdOrderCalcMargin uint32 = 202
	CmdOrderCalcProfit uint32 = 203
)

const (
	CmdMarketBookAdd     uint32 = 191
	CmdMarketBookRelease uint32 = 192
	CmdMarketBookGet     uint32 = 193
)
