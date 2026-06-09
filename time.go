package gomt5

import (
	"context"
	"fmt"
	"time"
)

func ToTime(unix int64) time.Time {
	return time.Unix(unix, 0).UTC()
}

func ToTimeMsc(msc int64) time.Time {
	return time.Unix(msc/1000, (msc%1000)*int64(time.Millisecond)).UTC()
}

func FromTime(t time.Time) int64 {
	return t.UTC().Unix()
}

func FromTimeMsc(t time.Time) int64 {
	return t.UTC().UnixMilli()
}

// TimeUTC returns TimeMsc as a time.Time labeled UTC. The instant is in the
// broker server's clock, so it equals true UTC only when the broker runs UTC.
func (t *Tick) TimeUTC() time.Time {
	return ToTimeMsc(t.TimeMsc)
}

// TimeUTC returns Time as a time.Time labeled UTC. The instant is in the
// broker server's clock, so it equals true UTC only when the broker runs UTC.
func (r *Rate) TimeUTC() time.Time {
	return ToTime(r.Time)
}

// TimeSetupUTC returns TimeSetupMsc as a time.Time labeled UTC
// (broker-clock instant, see the README section on time semantics).
func (o *Order) TimeSetupUTC() time.Time {
	return ToTimeMsc(o.TimeSetupMsc)
}

// TimeDoneUTC returns TimeDoneMsc as a time.Time labeled UTC
// (broker-clock instant, see the README section on time semantics).
func (o *Order) TimeDoneUTC() time.Time {
	return ToTimeMsc(o.TimeDoneMsc)
}

// TimeUTC returns TimeMsc as a time.Time labeled UTC
// (broker-clock instant, see the README section on time semantics).
func (p *Position) TimeUTC() time.Time {
	return ToTimeMsc(p.TimeMsc)
}

// TimeUpdateUTC returns TimeUpdateMsc as a time.Time labeled UTC
// (broker-clock instant, see the README section on time semantics).
func (p *Position) TimeUpdateUTC() time.Time {
	return ToTimeMsc(p.TimeUpdateMsc)
}

// TimeUTC returns TimeMsc as a time.Time labeled UTC
// (broker-clock instant, see the README section on time semantics).
func (d *Deal) TimeUTC() time.Time {
	return ToTimeMsc(d.TimeMsc)
}

// ClockSkew estimates brokerTime - localUTC from the open time of the
// current M1 bar of symbol, rounded to the nearest 30 minutes: it measures
// the broker timezone offset at that granularity, not fine clock drift, and
// assumes brokers sit on whole- or half-hour offsets.
//
// The symbol should be actively trading: while ticks arrive, the current
// bar lags the broker clock by less than a minute, so a healthy estimate
// sits within 90s below a half-hour multiple. Bars outside that window
// (market closed, no recent ticks) or implying an offset beyond +-14h are
// rejected with ErrStaleBar; staleness that happens to land within ~90s of
// a half-hour multiple still slips through undetected — the pipe protocol
// has no server-time command. Broker offsets typically shift with DST
// (UTC+2/UTC+3) twice a year, so the returned value is valid at call time
// only: do not cache it long-term or apply it to historical bars across a
// DST boundary.
func (c *Client) ClockSkew(ctx context.Context, symbol string) (time.Duration, error) {
	rates, err := c.CopyRatesFromPos(ctx, symbol, TimeframeM1, 0, 1)
	if err != nil {
		return 0, err
	}
	if len(rates) == 0 {
		return 0, fmt.Errorf("clock skew %s: %w", symbol, ErrNoBars)
	}
	raw := time.Until(ToTime(rates[0].Time))
	skew := raw.Round(30 * time.Minute)
	residual := raw - skew
	if residual < -90*time.Second || residual > 10*time.Second {
		return 0, fmt.Errorf("clock skew %s: bar %v off the nearest half-hour: %w", symbol, residual.Truncate(time.Second), ErrStaleBar)
	}
	if skew < -14*time.Hour || skew > 14*time.Hour {
		return 0, fmt.Errorf("clock skew %s: %v outside plausible offset range: %w", symbol, skew, ErrStaleBar)
	}
	return skew, nil
}
