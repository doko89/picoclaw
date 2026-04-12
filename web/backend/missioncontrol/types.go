// Package missioncontrol provides common SQL types.
//
// The types package provides convenience types for SQL NULL values.
package missioncontrol

import (
	"fmt"
	"time"
)

// SQLNullString wraps sql.NullString for convenience.
type SQLNullString struct {
	String string
	Valid  bool
}

// Scan implements sql.Scanner interface.
func (s *SQLNullString) Scan(value any) error {
	if value == nil {
		s.String, s.Valid = "", false
		return nil
	}
	s.Valid = true
	switch v := value.(type) {
	case []byte:
		s.String = string(v)
	case string:
		s.String = v
	default:
		return fmt.Errorf("unsupported type for NullString: %T", value)
	}
	return nil
}

// SQLNullTime wraps sql.NullTime for convenience.
type SQLNullTime struct {
	Time  time.Time
	Valid bool
}

// Scan implements sql.Scanner interface.
func (t *SQLNullTime) Scan(value any) error {
	if value == nil {
		t.Time, t.Valid = time.Time{}, false
		return nil
	}
	t.Valid = true
	switch v := value.(type) {
	case []byte:
		var err error
		t.Time, err = time.Parse(time.RFC3339, string(v))
		return err
	case string:
		var err error
		t.Time, err = time.Parse(time.RFC3339, v)
		return err
	case time.Time:
		t.Time = v
		return nil
	default:
		return fmt.Errorf("unsupported type for NullTime: %T", value)
	}
}

// SQLNullFloat64 wraps sql.NullFloat64 for convenience.
type SQLNullFloat64 struct {
	Float64 float64
	Valid   bool
}

// Scan implements sql.Scanner interface.
func (f *SQLNullFloat64) Scan(value any) error {
	if value == nil {
		f.Float64, f.Valid = 0, false
		return nil
	}
	f.Valid = true
	switch v := value.(type) {
	case float64:
		f.Float64 = v
		return nil
	case []byte:
		_, err := fmt.Sscanf(string(v), "%f", &f.Float64)
		f.Valid = err == nil
		return err
	case string:
		_, err := fmt.Sscanf(v, "%f", &f.Float64)
		f.Valid = err == nil
		return err
	default:
		return fmt.Errorf("unsupported type for NullFloat64: %T", value)
	}
}

// SQLNullInt64 wraps sql.NullInt64 for convenience.
type SQLNullInt64 struct {
	Int64 int64
	Valid bool
}

// Scan implements sql.Scanner interface.
func (i *SQLNullInt64) Scan(value any) error {
	if value == nil {
		i.Int64, i.Valid = 0, false
		return nil
	}
	i.Valid = true
	switch v := value.(type) {
	case int64:
		i.Int64 = v
		return nil
	case []byte:
		_, err := fmt.Sscanf(string(v), "%d", &i.Int64)
		i.Valid = err == nil
		return err
	case string:
		_, err := fmt.Sscanf(v, "%d", &i.Int64)
		i.Valid = err == nil
		return err
	default:
		return fmt.Errorf("unsupported type for NullInt64: %T", value)
	}
}
