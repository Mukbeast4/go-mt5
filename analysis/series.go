package analysis

import gomt5 "github.com/mukbeast4/go-mt5"

type RateSeries struct {
	rates []gomt5.Rate
}

func NewRateSeries(rates []gomt5.Rate) *RateSeries {
	return &RateSeries{rates: rates}
}

func (s *RateSeries) Len() int {
	return len(s.rates)
}

func (s *RateSeries) Rates() []gomt5.Rate {
	return s.rates
}

func (s *RateSeries) Opens() []float64 {
	out := make([]float64, len(s.rates))
	for i, r := range s.rates {
		out[i] = r.Open
	}
	return out
}

func (s *RateSeries) Closes() []float64 {
	out := make([]float64, len(s.rates))
	for i, r := range s.rates {
		out[i] = r.Close
	}
	return out
}

func (s *RateSeries) Highs() []float64 {
	out := make([]float64, len(s.rates))
	for i, r := range s.rates {
		out[i] = r.High
	}
	return out
}

func (s *RateSeries) Lows() []float64 {
	out := make([]float64, len(s.rates))
	for i, r := range s.rates {
		out[i] = r.Low
	}
	return out
}

func (s *RateSeries) Volumes() []float64 {
	out := make([]float64, len(s.rates))
	for i, r := range s.rates {
		out[i] = float64(r.TickVolume)
	}
	return out
}

func (s *RateSeries) Times() []int64 {
	out := make([]int64, len(s.rates))
	for i, r := range s.rates {
		out[i] = r.Time
	}
	return out
}
