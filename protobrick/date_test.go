package protobrick

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"google.golang.org/genproto/googleapis/type/date"
)

func TestValidateProtoDate(t *testing.T) {
	tests := []struct {
		name string
		date *date.Date
		want error
	}{
		{
			name: "ValidDate",
			date: &date.Date{Year: 2022, Month: 12, Day: 31},
			want: nil,
		},
		{
			name: "InvalidYear-1",
			date: &date.Date{Year: 10000, Month: 12, Day: 31},
			want: assert.AnError,
		},
		{
			name: "InvalidYear-2",
			date: &date.Date{Month: 12, Day: 31},
			want: assert.AnError,
		},
		{
			name: "InvalidMonth-1",
			date: &date.Date{Year: 2022, Month: 13, Day: 31},
			want: assert.AnError,
		},
		{
			name: "InvalidMonth-2",
			date: &date.Date{Year: 2022, Day: 31},
			want: assert.AnError,
		},
		{
			name: "InvalidDay-1",
			date: &date.Date{Year: 2022, Month: 12, Day: 32},
			want: assert.AnError,
		},
		{
			name: "InvalidDay-2",
			date: &date.Date{Year: 2022, Month: 12, Day: 32},
			want: assert.AnError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateProtoDate(tt.date)
			if tt.want == nil {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestConvertFromProtoDate(t *testing.T) {
	tests := []struct {
		name string
		date *date.Date
		want time.Time
	}{
		{
			name: "NotNilDate",
			date: &date.Date{Year: 2022, Month: 12, Day: 31},
			want: time.Date(2022, 12, 31, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "NilDate",
			date: nil,
			want: time.Date(0, 0, 0, 0, 0, 0, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ConvertFromProtoDate(tt.date)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestConvertToProtoDate(t *testing.T) {
	tests := []struct {
		name string
		time *time.Time
		want *date.Date
	}{
		{
			name: "ValidTime",
			time: func() *time.Time { t := time.Date(2022, 12, 31, 0, 0, 0, 0, time.UTC); return &t }(),
			want: &date.Date{Year: 2022, Month: 12, Day: 31},
		},
		{
			name: "NilTime",
			time: nil,
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ConvertToProtoDate(tt.time)
			assert.Equal(t, tt.want, got)
		})
	}
}
