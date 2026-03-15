package tradeutil

import gomt5 "github.com/mukbeast4/go-mt5"

type TradeOption func(*tradeOpts)

type tradeOpts struct {
	deviation  int
	magic      int64
	comment    string
	filling    gomt5.OrderFilling
	fillingSet bool
	sl         float64
	tp         float64
}

func WithDeviation(d int) TradeOption {
	return func(o *tradeOpts) { o.deviation = d }
}

func WithMagic(m int64) TradeOption {
	return func(o *tradeOpts) { o.magic = m }
}

func WithComment(c string) TradeOption {
	return func(o *tradeOpts) { o.comment = c }
}

func WithFilling(f gomt5.OrderFilling) TradeOption {
	return func(o *tradeOpts) {
		o.filling = f
		o.fillingSet = true
	}
}

func WithSLTP(sl, tp float64) TradeOption {
	return func(o *tradeOpts) {
		o.sl = sl
		o.tp = tp
	}
}

func applyOpts(opts []TradeOption) tradeOpts {
	var o tradeOpts
	o.deviation = 20
	for _, opt := range opts {
		opt(&o)
	}
	return o
}
