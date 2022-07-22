package value

import (
	"time"
)

// MS-DOS 16-bit "dos time" value
// Identical to the Time-part of a 32-Bit Windows Time+Date field
type DosTime struct {
	ts time.Time
}

func (dt DosTime) String() string {
	return dt.ts.Format("15:04:05")
}

func AsDosTime(v uint16) DosTime {
	hour := int(v >> 11)
	min := int((v >> 5) & 0x3f)
	sec := int((v & 0x1f) * 2)
	return DosTime{
		ts: time.Date(0, 0, 0, hour, min, sec, 0, time.UTC),
	}
}

// MS-DOS 16-bit "dos date" value.
// Identical to the Date-part of a 32-Bit Windows Time+Date field
type DosDate struct {
	ts time.Time
}

func (dt DosDate) String() string {
	return dt.ts.Format("2006-01-02")
}

func AsDosDate(v uint16) DosDate {
	day := int(v & 0x1f)
	month := time.Month((v >> 5 & 0x0f))
	year := int(1980 + (v >> 9))
	return DosDate{
		ts: time.Date(year, month, day, 0, 0, 0, 0, time.UTC),
	}
}

func AsDosTimeDate(v uint32) time.Time {
	date := uint16(v >> 16)
	day := int(date & 0x1f)
	month := time.Month((date >> 5 & 0x0f))
	year := int(1980 + (date >> 9))

	tod := uint16(v)
	hour := int(tod >> 11)
	min := int((tod >> 5) & 0x3f)
	sec := int((tod & 0x1f) * 2)
	return time.Date(year, month, day, hour, min, sec, 0, time.UTC)
}
