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

func (t Timeframe) String() string {
	switch t {
	case TimeframeM1:
		return "M1"
	case TimeframeM2:
		return "M2"
	case TimeframeM3:
		return "M3"
	case TimeframeM4:
		return "M4"
	case TimeframeM5:
		return "M5"
	case TimeframeM6:
		return "M6"
	case TimeframeM10:
		return "M10"
	case TimeframeM12:
		return "M12"
	case TimeframeM15:
		return "M15"
	case TimeframeM20:
		return "M20"
	case TimeframeM30:
		return "M30"
	case TimeframeH1:
		return "H1"
	case TimeframeH2:
		return "H2"
	case TimeframeH3:
		return "H3"
	case TimeframeH4:
		return "H4"
	case TimeframeH6:
		return "H6"
	case TimeframeH8:
		return "H8"
	case TimeframeH12:
		return "H12"
	case TimeframeD1:
		return "D1"
	case TimeframeW1:
		return "W1"
	case TimeframeMN1:
		return "MN1"
	default:
		return "Unknown"
	}
}

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

func (o OrderType) String() string {
	switch o {
	case OrderTypeBuy:
		return "Buy"
	case OrderTypeSell:
		return "Sell"
	case OrderTypeBuyLimit:
		return "BuyLimit"
	case OrderTypeSellLimit:
		return "SellLimit"
	case OrderTypeBuyStop:
		return "BuyStop"
	case OrderTypeSellStop:
		return "SellStop"
	case OrderTypeBuyStopLimit:
		return "BuyStopLimit"
	case OrderTypeSellStopLimit:
		return "SellStopLimit"
	case OrderTypeCloseBy:
		return "CloseBy"
	default:
		return "Unknown"
	}
}

type TradeAction int

const (
	TradeActionDeal    TradeAction = 1
	TradeActionPending TradeAction = 5
	TradeActionSLTP    TradeAction = 6
	TradeActionModify  TradeAction = 7
	TradeActionRemove  TradeAction = 8
	TradeActionCloseBy TradeAction = 10
)

func (a TradeAction) String() string {
	switch a {
	case TradeActionDeal:
		return "Deal"
	case TradeActionPending:
		return "Pending"
	case TradeActionSLTP:
		return "SLTP"
	case TradeActionModify:
		return "Modify"
	case TradeActionRemove:
		return "Remove"
	case TradeActionCloseBy:
		return "CloseBy"
	default:
		return "Unknown"
	}
}

type OrderFilling int

const (
	OrderFillingFOK    OrderFilling = 0
	OrderFillingIOC    OrderFilling = 1
	OrderFillingReturn OrderFilling = 2
	OrderFillingBOC    OrderFilling = 3
)

func (f OrderFilling) String() string {
	switch f {
	case OrderFillingFOK:
		return "FOK"
	case OrderFillingIOC:
		return "IOC"
	case OrderFillingReturn:
		return "Return"
	case OrderFillingBOC:
		return "BOC"
	default:
		return "Unknown"
	}
}

type OrderTime int

const (
	OrderTimeGTC       OrderTime = 0
	OrderTimeDay       OrderTime = 1
	OrderTimeSpecified OrderTime = 2
	OrderTimeSpecDay   OrderTime = 3
)

func (t OrderTime) String() string {
	switch t {
	case OrderTimeGTC:
		return "GTC"
	case OrderTimeDay:
		return "Day"
	case OrderTimeSpecified:
		return "Specified"
	case OrderTimeSpecDay:
		return "SpecDay"
	default:
		return "Unknown"
	}
}

type CopyTicksFlag int

const (
	CopyTicksAll   CopyTicksFlag = -1
	CopyTicksInfo  CopyTicksFlag = 1
	CopyTicksTrade CopyTicksFlag = 2
)

func (f CopyTicksFlag) String() string {
	switch f {
	case CopyTicksAll:
		return "All"
	case CopyTicksInfo:
		return "Info"
	case CopyTicksTrade:
		return "Trade"
	default:
		return "Unknown"
	}
}

type BookType int

const (
	BookTypeSell       BookType = 1
	BookTypeBuy        BookType = 2
	BookTypeSellMarket BookType = 3
	BookTypeBuyMarket  BookType = 4
)

func (b BookType) String() string {
	switch b {
	case BookTypeSell:
		return "Sell"
	case BookTypeBuy:
		return "Buy"
	case BookTypeSellMarket:
		return "SellMarket"
	case BookTypeBuyMarket:
		return "BuyMarket"
	default:
		return "Unknown"
	}
}

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

func (d DealType) String() string {
	switch d {
	case DealTypeBuy:
		return "Buy"
	case DealTypeSell:
		return "Sell"
	case DealTypeBalance:
		return "Balance"
	case DealTypeCredit:
		return "Credit"
	case DealTypeCharge:
		return "Charge"
	case DealTypeCorrection:
		return "Correction"
	case DealTypeBonus:
		return "Bonus"
	case DealTypeCommission:
		return "Commission"
	case DealTypeCommissionDaily:
		return "CommissionDaily"
	case DealTypeCommissionMonthly:
		return "CommissionMonthly"
	case DealTypeCommissionAgent:
		return "CommissionAgent"
	case DealTypeInterest:
		return "Interest"
	case DealTypeBuyCanceled:
		return "BuyCanceled"
	case DealTypeSellCanceled:
		return "SellCanceled"
	case DealTypeDividend:
		return "Dividend"
	case DealTypeDividendFranked:
		return "DividendFranked"
	case DealTypeTax:
		return "Tax"
	default:
		return "Unknown"
	}
}

type DealEntry int

const (
	DealEntryIn    DealEntry = 0
	DealEntryOut   DealEntry = 1
	DealEntryInOut DealEntry = 2
	DealEntryState DealEntry = 3
)

func (e DealEntry) String() string {
	switch e {
	case DealEntryIn:
		return "In"
	case DealEntryOut:
		return "Out"
	case DealEntryInOut:
		return "InOut"
	case DealEntryState:
		return "State"
	default:
		return "Unknown"
	}
}

type PositionType int

const (
	PositionTypeBuy  PositionType = 0
	PositionTypeSell PositionType = 1
)

func (p PositionType) String() string {
	switch p {
	case PositionTypeBuy:
		return "Buy"
	case PositionTypeSell:
		return "Sell"
	default:
		return "Unknown"
	}
}
