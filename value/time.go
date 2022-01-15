package value

import "time"

// MS-DOS 16-bit "dos time" value
type DosTime struct {
	ts time.Time
}

func (dt DosTime) String() string {
	return dt.ts.Format("15:04:05")
}

func asDosTime(v uint16) DosTime {
	hour := int(v >> 11)
	min := int((v >> 5) & 0x3f)
	sec := int((v & 0x1f) * 2)
	return DosTime{
		ts: time.Date(0, 0, 0, hour, min, sec, 0, time.UTC),
	}
}

// MS-DOS 16-bit "dos date" value
type DosDate struct {
	ts time.Time
}

func (dt DosDate) String() string {
	return dt.ts.Format("2006-01-02")
}

func asDosDate(v uint16) DosDate {
	day := int(v & 0x1f)
	month := time.Month((v >> 5 & 0x0f))
	year := int(1980 + (v >> 9))
	return DosDate{
		ts: time.Date(year, month, day, 0, 0, 0, 0, time.UTC),
	}
}
