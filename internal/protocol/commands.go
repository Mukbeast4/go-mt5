package protocol

const (
	CmdTerminalInfo      uint32 = 1
	CmdInitialize        uint32 = 4
	CmdCopyTicksFrom     uint32 = 104
	CmdCopyRatesFromPos  uint32 = 108
	CmdSetAPIVersion     uint32 = 109
	CmdPositionsTotal    uint32 = 120
	CmdOrdersTotal       uint32 = 130
	CmdHistoryOrdersTotal uint32 = 140
	CmdHistoryDealsTotal uint32 = 150
	CmdSymbolInfo        uint32 = 170
	CmdSymbolInfoTick    uint32 = 172
	CmdSymbolsTotal      uint32 = 173
	CmdTerminalInfoFull  uint32 = 180
	CmdAccountInfo       uint32 = 190
)
