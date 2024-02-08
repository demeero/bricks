package protobrick

import (
	"time"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"google.golang.org/genproto/googleapis/type/date"
)

// ValidateProtoDate validates a date.Date proto message.
func ValidateProtoDate(d *date.Date) error {
	return validation.Errors{
		"year":  validation.Validate(d.GetYear(), validation.Required, validation.Min(1), validation.Max(9999)),
		"month": validation.Validate(d.GetMonth(), validation.Required, validation.Min(1), validation.Max(12)),
		"day":   validation.Validate(d.GetDay(), validation.Required, validation.Min(1), validation.Max(31)),
	}.Filter()
}

// ConvertFromProtoDate converts a date.Date proto message to a time.Time.
func ConvertFromProtoDate(d *date.Date) time.Time {
	return time.Date(int(d.GetYear()), time.Month(d.GetMonth()), int(d.GetDay()), 0, 0, 0, 0, time.UTC)
}

// ConvertToProtoDate converts a time.Time to a date.Date proto message.
// If t is nil, it returns nil.
func ConvertToProtoDate(t *time.Time) *date.Date {
	if t == nil {
		return nil
	}
	return &date.Date{
		Year:  int32(t.Year()),
		Month: int32(t.Month()),
		Day:   int32(t.Day()),
	}
}
