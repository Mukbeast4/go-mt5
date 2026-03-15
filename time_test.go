package gomt5

import (
	"testing"
	"time"
)

func TestToTime(t *testing.T) {
	unix := int64(1710000000)
	result := ToTime(unix)

	if result.Location() != time.UTC {
		t.Errorf("expected UTC, got %v", result.Location())
	}
	if result.Unix() != unix {
		t.Errorf("expected unix %d, got %d", unix, result.Unix())
	}
}

func TestToTimeMsc(t *testing.T) {
	msc := int64(1710000000123)
	result := ToTimeMsc(msc)

	if result.Location() != time.UTC {
		t.Errorf("expected UTC, got %v", result.Location())
	}
	if result.UnixMilli() != msc {
		t.Errorf("expected msc %d, got %d", msc, result.UnixMilli())
	}
}

func TestFromTime(t *testing.T) {
	now := time.Date(2025, 3, 10, 12, 0, 0, 0, time.FixedZone("UTC+2", 2*3600))
	unix := FromTime(now)
	expected := now.UTC().Unix()

	if unix != expected {
		t.Errorf("expected %d, got %d", expected, unix)
	}
}

func TestFromTimeMsc(t *testing.T) {
	now := time.Date(2025, 3, 10, 12, 0, 0, 123000000, time.UTC)
	msc := FromTimeMsc(now)
	expected := now.UnixMilli()

	if msc != expected {
		t.Errorf("expected %d, got %d", expected, msc)
	}
}

func TestTickTimeUTC(t *testing.T) {
	tick := &Tick{TimeMsc: 1710000000123}
	result := tick.TimeUTC()

	if result.Location() != time.UTC {
		t.Error("expected UTC")
	}
	if result.UnixMilli() != 1710000000123 {
		t.Errorf("expected 1710000000123, got %d", result.UnixMilli())
	}
}

func TestRateTimeUTC(t *testing.T) {
	rate := &Rate{Time: 1710000000}
	result := rate.TimeUTC()

	if result.Location() != time.UTC {
		t.Error("expected UTC")
	}
	if result.Unix() != 1710000000 {
		t.Errorf("expected 1710000000, got %d", result.Unix())
	}
}

func TestDSTEdgeCases(t *testing.T) {
	dstSwitch := time.Date(2025, 3, 30, 1, 0, 0, 0, time.UTC)
	unix := FromTime(dstSwitch)
	roundtrip := ToTime(unix)

	if !roundtrip.Equal(dstSwitch) {
		t.Errorf("DST roundtrip failed: %v != %v", roundtrip, dstSwitch)
	}

	localDST := time.Date(2025, 3, 30, 3, 0, 0, 0, time.FixedZone("EET", 2*3600))
	unix2 := FromTime(localDST)
	result := ToTime(unix2)

	if result.Location() != time.UTC {
		t.Error("expected UTC after DST conversion")
	}
}
