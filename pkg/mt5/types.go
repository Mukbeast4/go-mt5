package mt5

type AccountInfo struct {
	Login          int64   `json:"login"`
	Name           string  `json:"name"`
	Server         string  `json:"server"`
	Currency       string  `json:"currency"`
	Balance        float64 `json:"balance"`
	Equity         float64 `json:"equity"`
	Margin         float64 `json:"margin"`
	FreeMargin     float64 `json:"free_margin"`
	MarginLevel    float64 `json:"margin_level"`
	Profit         float64 `json:"profit"`
	Leverage       int     `json:"leverage"`
	TradeMode      int     `json:"trade_mode"`
	LimitOrders    int     `json:"limit_orders"`
	TradeAllowed   bool    `json:"trade_allowed"`
	TradeExpert    bool    `json:"trade_expert"`
	CurrencyDigits int    `json:"currency_digits"`
}

type TerminalInfo struct {
	Build       int    `json:"build"`
	Name        string `json:"name"`
	Path        string `json:"path"`
	DataPath    string `json:"data_path"`
	CommonPath  string `json:"common_path"`
	Connected   bool   `json:"connected"`
	Community   bool   `json:"community_connection"`
	TradeAllowed bool  `json:"trade_allowed"`
	DLLsAllowed bool   `json:"dlls_allowed"`
}

type VersionInfo struct {
	Version    string `json:"version"`
	Build      int    `json:"build"`
	BuildDate  string `json:"build_date"`
	EAVersion  string `json:"ea_version"`
}

type SymbolInfo struct {
	Name            string  `json:"name"`
	Description     string  `json:"description"`
	Path            string  `json:"path"`
	CurrencyBase    string  `json:"currency_base"`
	CurrencyProfit  string  `json:"currency_profit"`
	CurrencyMargin  string  `json:"currency_margin"`
	Bid             float64 `json:"bid"`
	Ask             float64 `json:"ask"`
	Last            float64 `json:"last"`
	Volume          int64   `json:"volume"`
	VolumeMin       float64 `json:"volume_min"`
	VolumeMax       float64 `json:"volume_max"`
	VolumeStep      float64 `json:"volume_step"`
	Point           float64 `json:"point"`
	Digits          int     `json:"digits"`
	Spread          int     `json:"spread"`
	SpreadFloat     bool    `json:"spread_float"`
	TradeMode       int     `json:"trade_mode"`
	TradeCalcMode   int     `json:"trade_calc_mode"`
	ContractSize    float64 `json:"contract_size"`
	TickValue       float64 `json:"tick_value"`
	TickSize        float64 `json:"tick_size"`
	SwapLong        float64 `json:"swap_long"`
	SwapShort       float64 `json:"swap_short"`
	Visible         bool    `json:"visible"`
	SessionDeals    int64   `json:"session_deals"`
	SessionBuyOrders int64  `json:"session_buy_orders"`
	SessionSellOrders int64 `json:"session_sell_orders"`
}

type Tick struct {
	Symbol  string  `json:"symbol,omitempty"`
	Time    int64   `json:"time"`
	TimeMsc int64   `json:"time_msc"`
	Bid     float64 `json:"bid"`
	Ask     float64 `json:"ask"`
	Last    float64 `json:"last"`
	Volume  int64   `json:"volume"`
	Flags   int     `json:"flags"`
}

type Rate struct {
	Time       int64   `json:"time"`
	Open       float64 `json:"open"`
	High       float64 `json:"high"`
	Low        float64 `json:"low"`
	Close      float64 `json:"close"`
	TickVolume int64   `json:"tick_volume"`
	Spread     int     `json:"spread"`
	RealVolume int64   `json:"real_volume"`
}

type BookEntry struct {
	Type       BookType `json:"type"`
	Price      float64  `json:"price"`
	Volume     int64    `json:"volume"`
	VolumeReal float64  `json:"volume_real"`
}

type TradeRequest struct {
	Action      TradeAction  `json:"action"`
	Magic       int64        `json:"magic,omitempty"`
	Order       int64        `json:"order,omitempty"`
	Symbol      string       `json:"symbol,omitempty"`
	Volume      float64      `json:"volume,omitempty"`
	Price       float64      `json:"price,omitempty"`
	StopLimit   float64      `json:"stoplimit,omitempty"`
	SL          float64      `json:"sl,omitempty"`
	TP          float64      `json:"tp,omitempty"`
	Deviation   int          `json:"deviation,omitempty"`
	Type        OrderType    `json:"type,omitempty"`
	TypeFilling OrderFilling `json:"type_filling,omitempty"`
	TypeTime    OrderTime    `json:"type_time,omitempty"`
	Expiration  int64        `json:"expiration,omitempty"`
	Comment     string       `json:"comment,omitempty"`
	Position    int64        `json:"position,omitempty"`
	PositionBy  int64        `json:"position_by,omitempty"`
}

type TradeResult struct {
	Retcode    int     `json:"retcode"`
	Deal       int64   `json:"deal"`
	Order      int64   `json:"order"`
	Volume     float64 `json:"volume"`
	Price      float64 `json:"price"`
	Bid        float64 `json:"bid"`
	Ask        float64 `json:"ask"`
	Comment    string  `json:"comment"`
	RequestID  int     `json:"request_id"`
	RetcodeExt int     `json:"retcode_external"`
}

type CheckResult struct {
	Retcode     int     `json:"retcode"`
	Balance     float64 `json:"balance"`
	Equity      float64 `json:"equity"`
	Profit      float64 `json:"profit"`
	Margin      float64 `json:"margin"`
	MarginFree  float64 `json:"margin_free"`
	MarginLevel float64 `json:"margin_level"`
	Comment     string  `json:"comment"`
}

type Order struct {
	Ticket         int64       `json:"ticket"`
	TimeDone       int64       `json:"time_done"`
	TimeDoneMsc    int64       `json:"time_done_msc"`
	TimeSetup      int64       `json:"time_setup"`
	TimeSetupMsc   int64       `json:"time_setup_msc"`
	Type           OrderType   `json:"type"`
	TypeFilling    OrderFilling `json:"type_filling"`
	TypeTime       OrderTime   `json:"type_time"`
	Magic          int64       `json:"magic"`
	PositionID     int64       `json:"position_id"`
	VolumeInitial  float64     `json:"volume_initial"`
	VolumeCurrent  float64     `json:"volume_current"`
	PriceOpen      float64     `json:"price_open"`
	PriceCurrent   float64     `json:"price_current"`
	PriceSL        float64     `json:"price_sl"`
	PriceTP        float64     `json:"price_tp"`
	PriceStopLimit float64     `json:"price_stoplimit"`
	Symbol         string      `json:"symbol"`
	Comment        string      `json:"comment"`
	ExternalID     string      `json:"external_id"`
}

type Position struct {
	Ticket       int64        `json:"ticket"`
	Time         int64        `json:"time"`
	TimeMsc      int64        `json:"time_msc"`
	TimeUpdate   int64        `json:"time_update"`
	TimeUpdateMsc int64       `json:"time_update_msc"`
	Type         PositionType `json:"type"`
	Magic        int64        `json:"magic"`
	Identifier   int64        `json:"identifier"`
	Volume       float64      `json:"volume"`
	PriceOpen    float64      `json:"price_open"`
	PriceCurrent float64      `json:"price_current"`
	PriceSL      float64      `json:"price_sl"`
	PriceTP      float64      `json:"price_tp"`
	Swap         float64      `json:"swap"`
	Profit       float64      `json:"profit"`
	Symbol       string       `json:"symbol"`
	Comment      string       `json:"comment"`
	ExternalID   string       `json:"external_id"`
}

type Deal struct {
	Ticket     int64    `json:"ticket"`
	Order      int64    `json:"order"`
	Time       int64    `json:"time"`
	TimeMsc    int64    `json:"time_msc"`
	Type       DealType `json:"type"`
	Entry      DealEntry `json:"entry"`
	Magic      int64    `json:"magic"`
	PositionID int64    `json:"position_id"`
	Volume     float64  `json:"volume"`
	Price      float64  `json:"price"`
	Commission float64  `json:"commission"`
	Swap       float64  `json:"swap"`
	Profit     float64  `json:"profit"`
	Fee        float64  `json:"fee"`
	Symbol     string   `json:"symbol"`
	Comment    string   `json:"comment"`
	ExternalID string   `json:"external_id"`
}
