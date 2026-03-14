package gomt5

type Timeframe int

const (
	TimeframeM1  Timeframe = 1
	TimeframeM2  Timeframe = 2
	TimeframeM3  Timeframe = 3
	TimeframeM4  Timeframe = 4
	TimeframeM5  Timeframe = 5
	TimeframeM6  Timeframe = 6
	TimeframeM10 Timeframe = 10
	TimeframeM12 Timeframe = 12
	TimeframeM15 Timeframe = 15
	TimeframeM20 Timeframe = 20
	TimeframeM30 Timeframe = 30
	TimeframeH1  Timeframe = 16385
	TimeframeH2  Timeframe = 16386
	TimeframeH3  Timeframe = 16387
	TimeframeH4  Timeframe = 16388
	TimeframeH6  Timeframe = 16390
	TimeframeH8  Timeframe = 16392
	TimeframeH12 Timeframe = 16396
	TimeframeD1  Timeframe = 16408
	TimeframeW1  Timeframe = 32769
	TimeframeMN1 Timeframe = 49153
)

type OrderType int

const (
	OrderTypeBuy           OrderType = 0
	OrderTypeSell          OrderType = 1
	OrderTypeBuyLimit      OrderType = 2
	OrderTypeSellLimit     OrderType = 3
	OrderTypeBuyStop       OrderType = 4
	OrderTypeSellStop      OrderType = 5
	OrderTypeBuyStopLimit  OrderType = 6
	OrderTypeSellStopLimit OrderType = 7
	OrderTypeCloseBy       OrderType = 8
)

type TradeAction int

const (
	TradeActionDeal    TradeAction = 1
	TradeActionPending TradeAction = 5
	TradeActionSLTP    TradeAction = 6
	TradeActionModify  TradeAction = 7
	TradeActionRemove  TradeAction = 8
	TradeActionCloseBy TradeAction = 10
)

type OrderFilling int

const (
	OrderFillingFOK    OrderFilling = 0
	OrderFillingIOC    OrderFilling = 1
	OrderFillingReturn OrderFilling = 2
	OrderFillingBOC    OrderFilling = 3
)

type OrderTime int

const (
	OrderTimeGTC       OrderTime = 0
	OrderTimeDay       OrderTime = 1
	OrderTimeSpecified OrderTime = 2
	OrderTimeSpecDay   OrderTime = 3
)

type CopyTicksFlag int

const (
	CopyTicksAll   CopyTicksFlag = -1
	CopyTicksInfo  CopyTicksFlag = 1
	CopyTicksTrade CopyTicksFlag = 2
)

type BookType int

const (
	BookTypeSell       BookType = 1
	BookTypeBuy        BookType = 2
	BookTypeSellMarket BookType = 3
	BookTypeBuyMarket  BookType = 4
)

type DealType int

const (
	DealTypeBuy               DealType = 0
	DealTypeSell              DealType = 1
	DealTypeBalance           DealType = 2
	DealTypeCredit            DealType = 3
	DealTypeCharge            DealType = 4
	DealTypeCorrection        DealType = 5
	DealTypeBonus             DealType = 6
	DealTypeCommission        DealType = 7
	DealTypeCommissionDaily   DealType = 8
	DealTypeCommissionMonthly DealType = 9
	DealTypeCommissionAgent   DealType = 10
	DealTypeInterest          DealType = 11
	DealTypeBuyCanceled       DealType = 12
	DealTypeSellCanceled      DealType = 13
	DealTypeDividend          DealType = 14
	DealTypeDividendFranked   DealType = 15
	DealTypeTax               DealType = 16
)

type DealEntry int

const (
	DealEntryIn    DealEntry = 0
	DealEntryOut   DealEntry = 1
	DealEntryInOut DealEntry = 2
	DealEntryState DealEntry = 3
)

type PositionType int

const (
	PositionTypeBuy  PositionType = 0
	PositionTypeSell PositionType = 1
)
