package analysis

import (
	"fmt"
	"io"
	"time"

	gomt5 "github.com/mukbeast4/go-mt5"
)

func ToCSV(w io.Writer, rates []gomt5.Rate) error {
	if _, err := fmt.Fprintln(w, "time,open,high,low,close,tick_volume,spread,real_volume"); err != nil {
		return err
	}
	for _, r := range rates {
		t := time.Unix(r.Time, 0).UTC().Format(time.RFC3339)
		if _, err := fmt.Fprintf(w, "%s,%f,%f,%f,%f,%d,%d,%d\n",
			t, r.Open, r.High, r.Low, r.Close, r.TickVolume, r.Spread, r.RealVolume); err != nil {
			return err
		}
	}
	return nil
}
