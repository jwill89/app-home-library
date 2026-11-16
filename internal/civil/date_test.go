package civil

import (
	"testing"
	"time"
)

func TestDateText(t *testing.T) {
	var d Date
	if err := d.UnmarshalText([]byte("2026-11-07")); err != nil || d != "2026-11-07" {
		t.Errorf("read %q, err %v", d, err)
	}
	if b, err := d.MarshalText(); err != nil || string(b) != "2026-11-07" {
		t.Errorf("wrote %q, err %v", b, err)
	}
	if err := d.UnmarshalText([]byte("7 Nov 2026")); err == nil {
		t.Error("read a day in another form")
	}
	if err := d.UnmarshalText(nil); err != nil || d != "" {
		t.Errorf("nothing read as %q, err %v", d, err)
	}
	if s := (Date("")).Schema(nil); s.Type != "string" || s.Format != "date" {
		t.Errorf("schema %+v", s)
	}
}

func TestDateScanAndValue(t *testing.T) {
	var d Date
	if err := d.Scan(time.Date(2026, 11, 7, 0, 0, 0, 0, time.UTC)); err != nil || d != "2026-11-07" {
		t.Errorf("from the driver's time: %q, err %v", d, err)
	}
	if err := d.Scan(nil); err != nil || d != "" {
		t.Errorf("from NULL: %q, err %v", d, err)
	}
	if err := d.Scan(42); err == nil {
		t.Error("read a number as a day")
	}
	if v, err := Date("").Value(); err != nil || v != nil {
		t.Errorf("the empty day wrote %v, err %v", v, err)
	}
	if v, err := Date("2026-11-07").Value(); err != nil || v != "2026-11-07" {
		t.Errorf("wrote %v, err %v", v, err)
	}
	if _, err := Date("2026-02-30").Value(); err == nil {
		t.Error("wrote a day that isn't one")
	}
}
