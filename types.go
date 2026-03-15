package gomt5

type AccountInfo struct {
	Login             int64   `json:"login"`
	TradeMode         int64   `json:"trade_mode"`
	Leverage          int64   `json:"leverage"`
	LimitOrders       int64   `json:"limit_orders"`
	MarginSOMode      int64   `json:"margin_so_mode"`
	TradeAllowed      bool    `json:"trade_allowed"`
	TradeExpert       bool    `json:"trade_expert"`
	MarginMode        int64   `json:"margin_mode"`
	CurrencyDigits    int64   `json:"currency_digits"`
	FIFOClose         bool    `json:"fifo_close"`
	Balance           float64 `json:"balance"`
	Credit            float64 `json:"credit"`
	Profit            float64 `json:"profit"`
	Equity            float64 `json:"equity"`
	Margin            float64 `json:"margin"`
	FreeMargin        float64 `json:"free_margin"`
	MarginLevel       float64 `json:"margin_level"`
	MarginSOCall      float64 `json:"margin_so_call"`
	MarginSOSO        float64 `json:"margin_so_so"`
	MarginInitial     float64 `json:"margin_initial"`
	MarginMaintenance float64 `json:"margin_maintenance"`
	Assets            float64 `json:"assets"`
	Liabilities       float64 `json:"liabilities"`
	CommissionBlocked float64 `json:"commission_blocked"`
	Name              string  `json:"name"`
	Server            string  `json:"server"`
	Currency          string  `json:"currency"`
	Company           string  `json:"company"`
}

type TerminalInfo struct {
	CommunityAccount     bool    `json:"community_account"`
	CommunityConnection  bool    `json:"community_connection"`
	Connected            bool    `json:"connected"`
	DLLsAllowed          bool    `json:"dlls_allowed"`
	TradeAllowed         bool    `json:"trade_allowed"`
	TradeAPIDisabled     bool    `json:"tradeapi_disabled"`
	EmailEnabled         bool    `json:"email_enabled"`
	FTPEnabled           bool    `json:"ftp_enabled"`
	NotificationsEnabled bool    `json:"notifications_enabled"`
	MQID                 bool    `json:"mqid"`
	Build                int64   `json:"build"`
	MaxBars              int64   `json:"maxbars"`
	CodePage             int64   `json:"codepage"`
	PingLast             int64   `json:"ping_last"`
	CommunityBalance     float64 `json:"community_balance"`
	Retransmission       float64 `json:"retransmission"`
	Company              string  `json:"company"`
	Name                 string  `json:"name"`
	Language             string  `json:"language"`
	Path                 string  `json:"path"`
	DataPath             string  `json:"data_path"`
	CommonDataPath       string  `json:"commondata_path"`
}

type VersionInfo struct {
	Version   int
	Build     int
	BuildDate string
}

type SymbolInfo struct {
	Custom                  bool    `json:"custom"`
	ChartMode               int64   `json:"chart_mode"`
	Select                  bool    `json:"select"`
	Visible                 bool    `json:"visible"`
	SessionDeals            int64   `json:"session_deals"`
	SessionBuyOrders        int64   `json:"session_buy_orders"`
	SessionSellOrders       int64   `json:"session_sell_orders"`
	Volume                  int64   `json:"volume"`
	VolumeHigh              int64   `json:"volumehigh"`
	VolumeLow               int64   `json:"volumelow"`
	Time                    int64   `json:"time"`
	Digits                  int64   `json:"digits"`
	Spread                  int64   `json:"spread"`
	SpreadFloat             bool    `json:"spread_float"`
	TicksBookDepth          int64   `json:"ticks_bookdepth"`
	TradeCalcMode           int64   `json:"trade_calc_mode"`
	TradeMode               int64   `json:"trade_mode"`
	StartTime               int64   `json:"start_time"`
	ExpirationTime          int64   `json:"expiration_time"`
	TradeStopsLevel         int64   `json:"trade_stops_level"`
	TradeFreezeLevel        int64   `json:"trade_freeze_level"`
	TradeExeMode            int64   `json:"trade_exemode"`
	SwapMode                int64   `json:"swap_mode"`
	SwapRollover3Days       int64   `json:"swap_rollover3days"`
	MarginHedgedUseLeg      bool    `json:"margin_hedged_use_leg"`
	ExpirationMode          int64   `json:"expiration_mode"`
	FillingMode             int64   `json:"filling_mode"`
	OrderMode               int64   `json:"order_mode"`
	OrderGTCMode            int64   `json:"order_gtc_mode"`
	OptionMode              int64   `json:"option_mode"`
	OptionRight             int64   `json:"option_right"`
	Bid                     float64 `json:"bid"`
	BidHigh                 float64 `json:"bidhigh"`
	BidLow                  float64 `json:"bidlow"`
	Ask                     float64 `json:"ask"`
	AskHigh                 float64 `json:"askhigh"`
	AskLow                  float64 `json:"asklow"`
	Last                    float64 `json:"last"`
	LastHigh                float64 `json:"lasthigh"`
	LastLow                 float64 `json:"lastlow"`
	VolumeReal              float64 `json:"volume_real"`
	VolumeHighReal          float64 `json:"volumehigh_real"`
	VolumeLowReal           float64 `json:"volumelow_real"`
	OptionStrike            float64 `json:"option_strike"`
	Point                   float64 `json:"point"`
	TradeTickValue          float64 `json:"trade_tick_value"`
	TradeTickValueProfit    float64 `json:"trade_tick_value_profit"`
	TradeTickValueLoss      float64 `json:"trade_tick_value_loss"`
	TradeTickSize           float64 `json:"trade_tick_size"`
	TradeContractSize       float64 `json:"trade_contract_size"`
	TradeAccruedInterest    float64 `json:"trade_accrued_interest"`
	TradeFaceValue          float64 `json:"trade_face_value"`
	TradeLiquidityRate      float64 `json:"trade_liquidity_rate"`
	VolumeMin               float64 `json:"volume_min"`
	VolumeMax               float64 `json:"volume_max"`
	VolumeStep              float64 `json:"volume_step"`
	VolumeLimit             float64 `json:"volume_limit"`
	SwapLong                float64 `json:"swap_long"`
	SwapShort               float64 `json:"swap_short"`
	MarginInitial           float64 `json:"margin_initial"`
	MarginMaintenance       float64 `json:"margin_maintenance"`
	SessionVolume           float64 `json:"session_volume"`
	SessionTurnover         float64 `json:"session_turnover"`
	SessionInterest         float64 `json:"session_interest"`
	SessionBuyOrdersVolume  float64 `json:"session_buy_orders_volume"`
	SessionSellOrdersVolume float64 `json:"session_sell_orders_volume"`
	SessionOpen             float64 `json:"session_open"`
	SessionClose            float64 `json:"session_close"`
	SessionAW               float64 `json:"session_aw"`
	SessionPriceSettlement  float64 `json:"session_price_settlement"`
	SessionPriceLimitMin    float64 `json:"session_price_limit_min"`
	SessionPriceLimitMax    float64 `json:"session_price_limit_max"`
	MarginHedged            float64 `json:"margin_hedged"`
	PriceChange             float64 `json:"price_change"`
	PriceVolatility         float64 `json:"price_volatility"`
	PriceTheoretical        float64 `json:"price_theoretical"`
	PriceGreeksDelta        float64 `json:"price_greeks_delta"`
	PriceGreeksTheta        float64 `json:"price_greeks_theta"`
	PriceGreeksGamma        float64 `json:"price_greeks_gamma"`
	PriceGreeksVega         float64 `json:"price_greeks_vega"`
	PriceGreeksRho          float64 `json:"price_greeks_rho"`
	PriceGreeksOmega        float64 `json:"price_greeks_omega"`
	PriceSensitivity        float64 `json:"price_sensitivity"`
	Basis                   string  `json:"basis"`
	Category                string  `json:"category"`
	CurrencyBase            string  `json:"currency_base"`
	CurrencyProfit          string  `json:"currency_profit"`
	CurrencyMargin          string  `json:"currency_margin"`
	Bank                    string  `json:"bank"`
	Description             string  `json:"description"`
	Exchange                string  `json:"exchange"`
	Formula                 string  `json:"formula"`
	ISIN                    string  `json:"isin"`
	SymbolName              string  `json:"name"`
	Page                    string  `json:"page"`
	Path                    string  `json:"path"`
}

type Tick struct {
	Time    int64   `json:"time"`
	Bid     float64 `json:"bid"`
	Ask     float64 `json:"ask"`
	Last    float64 `json:"last"`
	Volume  uint64  `json:"volume"`
	TimeMsc int64   `json:"time_msc"`
	Flags   uint32  `json:"flags"`
}

type Rate struct {
	Time       int64   `json:"time"`
	Open       float64 `json:"open"`
	High       float64 `json:"high"`
	Low        float64 `json:"low"`
	Close      float64 `json:"close"`
	TickVolume int64   `json:"tick_volume"`
	Spread     int32   `json:"spread"`
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
	Retcode    uint32  `json:"retcode"`
	Deal       int64   `json:"deal"`
	Order      int64   `json:"order"`
	Volume     float64 `json:"volume"`
	Price      float64 `json:"price"`
	Bid        float64 `json:"bid"`
	Ask        float64 `json:"ask"`
	Comment    string  `json:"comment"`
	RequestID  uint32  `json:"request_id"`
	RetcodeExt int32   `json:"retcode_external"`
}

func (r *TradeResult) IsOK() bool {
	return r.Retcode == RetcodeDone || r.Retcode == RetcodeOK
}

type CheckResult struct {
	Retcode     uint32  `json:"retcode"`
	Balance     float64 `json:"balance"`
	Equity      float64 `json:"equity"`
	Profit      float64 `json:"profit"`
	Margin      float64 `json:"margin"`
	MarginFree  float64 `json:"margin_free"`
	MarginLevel float64 `json:"margin_level"`
	Comment     string  `json:"comment"`
}

type Order struct {
	Ticket         int64        `json:"ticket"`
	TimeSetup      int64        `json:"time_setup"`
	TimeSetupMsc   int64        `json:"time_setup_msc"`
	TimeDone       int64        `json:"time_done"`
	TimeDoneMsc    int64        `json:"time_done_msc"`
	TimeExpiration int64        `json:"time_expiration"`
	Type           OrderType    `json:"type"`
	TypeTime       OrderTime    `json:"type_time"`
	TypeFilling    OrderFilling `json:"type_filling"`
	State          int64        `json:"state"`
	Magic          int64        `json:"magic"`
	PositionID     int64        `json:"position_id"`
	PositionByID   int64        `json:"position_by_id"`
	Reason         int64        `json:"reason"`
	VolumeInitial  float64      `json:"volume_initial"`
	VolumeCurrent  float64      `json:"volume_current"`
	PriceOpen      float64      `json:"price_open"`
	PriceCurrent   float64      `json:"price_current"`
	PriceSL        float64      `json:"price_sl"`
	PriceTP        float64      `json:"price_tp"`
	PriceStopLimit float64      `json:"price_stoplimit"`
	Symbol         string       `json:"symbol"`
	Comment        string       `json:"comment"`
	ExternalID     string       `json:"external_id"`
}

type Position struct {
	Ticket        int64        `json:"ticket"`
	Time          int64        `json:"time"`
	TimeMsc       int64        `json:"time_msc"`
	TimeUpdate    int64        `json:"time_update"`
	TimeUpdateMsc int64        `json:"time_update_msc"`
	Type          PositionType `json:"type"`
	Magic         int64        `json:"magic"`
	Identifier    int64        `json:"identifier"`
	Reason        int64        `json:"reason"`
	Volume        float64      `json:"volume"`
	PriceOpen     float64      `json:"price_open"`
	PriceCurrent  float64      `json:"price_current"`
	PriceSL       float64      `json:"price_sl"`
	PriceTP       float64      `json:"price_tp"`
	Swap          float64      `json:"swap"`
	Profit        float64      `json:"profit"`
	Symbol        string       `json:"symbol"`
	Comment       string       `json:"comment"`
	ExternalID    string       `json:"external_id"`
}

type Deal struct {
	Ticket     int64     `json:"ticket"`
	Order      int64     `json:"order"`
	Time       int64     `json:"time"`
	TimeMsc    int64     `json:"time_msc"`
	Type       DealType  `json:"type"`
	Entry      DealEntry `json:"entry"`
	Magic      int64     `json:"magic"`
	PositionID int64     `json:"position_id"`
	Reason     int64     `json:"reason"`
	Volume     float64   `json:"volume"`
	Price      float64   `json:"price"`
	Commission float64   `json:"commission"`
	Swap       float64   `json:"swap"`
	Profit     float64   `json:"profit"`
	Fee        float64   `json:"fee"`
	Symbol     string    `json:"symbol"`
	Comment    string    `json:"comment"`
	ExternalID string    `json:"external_id"`
}

type OrderFilter struct {
	Symbol string
	Ticket int64
	Group  string
}

type PositionFilter struct {
	Symbol string
	Ticket int64
	Group  string
}

type HistoryFilter struct {
	DateFrom int64
	DateTo   int64
	Symbol   string
	Ticket   int64
	Group    string
}
