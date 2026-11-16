// Package civil holds a day on the calendar, with no time of day and no zone: the day a book
// was lent, read, bought or put aside.
package civil

import (
	"database/sql/driver"
	"fmt"
	"time"

	"github.com/danielgtaylor/huma/v2"
)

// Date is a DATE column's value: a calendar day, written 2026-11-07. The empty Date is a day
// not set, which the column holds as NULL.
type Date string

// Scan reads a DATE column. The driver hands a day over as midnight UTC, of which only the
// day is kept.
func (d *Date) Scan(src any) error {
	switch v := src.(type) {
	case nil:
		*d = ""
		return nil
	case time.Time:
		*d = Date(v.UTC().Format(time.DateOnly))
		return nil
	case string:
		return d.UnmarshalText([]byte(v))
	case []byte:
		return d.UnmarshalText(v)
	default:
		return fmt.Errorf("date: cannot read %T", src)
	}
}

// Value writes the day as 2026-11-07, and the empty Date as NULL.
func (d Date) Value() (driver.Value, error) {
	if d == "" {
		return nil, nil
	}
	t, err := time.Parse(time.DateOnly, string(d))
	if err != nil {
		return nil, fmt.Errorf("date %q: %w", string(d), err)
	}
	return t.Format(time.DateOnly), nil
}

// MarshalText writes the day as 2026-11-07, or nothing for a day not set.
func (d Date) MarshalText() ([]byte, error) {
	if d == "" {
		return []byte{}, nil
	}
	t, err := time.Parse(time.DateOnly, string(d))
	if err != nil {
		return nil, fmt.Errorf("date %q: %w", string(d), err)
	}
	return []byte(t.Format(time.DateOnly)), nil
}

// UnmarshalText reads a day written 2026-11-07, or nothing for a day not set.
func (d *Date) UnmarshalText(b []byte) error {
	if len(b) == 0 {
		*d = ""
		return nil
	}
	t, err := time.Parse(time.DateOnly, string(b))
	if err != nil {
		return fmt.Errorf("date %q: %w", string(b), err)
	}
	*d = Date(t.Format(time.DateOnly))
	return nil
}

// Schema describes the day to the OpenAPI document as an RFC 3339 full-date.
func (Date) Schema(huma.Registry) *huma.Schema {
	return &huma.Schema{Type: huma.TypeString, Format: "date"}
}

var _ huma.SchemaProvider = Date("")
