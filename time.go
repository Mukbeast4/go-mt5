package gomt5

import "time"

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

func (t *Tick) TimeUTC() time.Time {
	return ToTimeMsc(t.TimeMsc)
}

func (r *Rate) TimeUTC() time.Time {
	return ToTime(r.Time)
}

func (o *Order) TimeSetupUTC() time.Time {
	return ToTimeMsc(o.TimeSetupMsc)
}

func (o *Order) TimeDoneUTC() time.Time {
	return ToTimeMsc(o.TimeDoneMsc)
}

func (p *Position) TimeUTC() time.Time {
	return ToTimeMsc(p.TimeMsc)
}

func (p *Position) TimeUpdateUTC() time.Time {
	return ToTimeMsc(p.TimeUpdateMsc)
}

func (d *Deal) TimeUTC() time.Time {
	return ToTimeMsc(d.TimeMsc)
}
