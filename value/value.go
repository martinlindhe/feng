package value

import (
	"encoding/hex"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
)

var (
	bitExpressionRE = regexp.MustCompile(`b([01_]+)`)
)

// parses an "structs" data value (used by structs parser)
func ParseDataPattern(s string) (DataPattern, error) {
	dp := DataPattern{}
	if s == "??" {
		return dp, nil
	}
	value, err := ParseDataString(s)
	dp.Pattern = value
	dp.Known = true
	return dp, err
}

type DataPattern struct {
	// if false, data value was described as "??"
	Known bool

	// if true, holds the expected pattern
	Pattern []byte

	// holds the value (for custom values like in "endian: big")
	Value string
}

// parses "00 0a 0b" or "c'aazz'" into a byte array
func ParseDataString(s string) ([]byte, error) {

	var err error
	prev := ""
	for {
		prev = s
		s, err = replaceNextASCIITag(s)
		if err != nil {
			return nil, err
		}
		s, err = replaceNextBitTag(s)
		if err != nil {
			return nil, err
		}

		if s == prev {
			break
		}
	}

	s = strings.ReplaceAll(s, " ", "")
	res, err := hex.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("hex decode '%s' failed: %v", s, err)
	}
	return res, nil
}

// find next b1010_1010 and replace with hex
func replaceNextBitTag(s string) (string, error) {

	matches := bitExpressionRE.FindStringSubmatch(s)
	if len(matches) == 0 {
		return s, nil
	}

	idx := bitExpressionRE.FindStringSubmatchIndex(s)

	m := strings.ReplaceAll(matches[1], "_", "")
	i, err := strconv.ParseInt(m, 2, 64)
	if err != nil {
		return "", err
	}

	if i > 255 {
		panic("big binary number unhandled")
	}

	s = s[0:idx[0]] + fmt.Sprintf("%02x", i) + s[idx[1]:]

	return s, nil
}

// find next ascii between c'' characters and replace with hex
func replaceNextASCIITag(s string) (string, error) {
	p1 := strings.Index(s, "c'")
	if p1 == -1 {
		return s, nil
	}
	block := s[p1+len("c'"):]
	p2 := strings.Index(block, "'")
	if p2 == -1 {
		return "", fmt.Errorf("missing closing ' delimiter")
	}
	block = block[:p2]

	tmp, err := asciiToHexString(block)
	if err != nil {
		return "", err
	}
	s = s[0:p1] + tmp + s[p1+len("c'")+p2+1:]
	return s, nil
}

func asciiToHexString(s string) (string, error) {
	res := ""
	for _, r := range s {
		v := int(r)
		if v < 0 || v > 127 {
			return "", fmt.Errorf("unexpected rune value (not ascii) '%v'", r)
		}
		//fmt.Printf("%c => ascii %02x\n", r, v)
		res += fmt.Sprintf("%02x", v)
	}
	return res, nil
}

func ParseDataField(s string) (DataField, error) {

	df := DataField{}

	p1 := strings.Index(s, "[")
	p2 := strings.Index(s, "]")
	if p2 < p1 {
		return df, fmt.Errorf("invalid range syntax '%s'", s)
	}

	// slice format: kind[] label
	if p2 == p1+1 {
		df.Slice = true
	}

	space := strings.Index(s, " ")
	if space == -1 {
		// single token like "endian"
		df.Kind = s
	} else if p1 >= 0 {
		// ranged format: kind[range] label
		df.Kind = strings.TrimSpace(s[0:p1])
		df.Range = strings.TrimSpace(s[p1+1 : p2])
		df.Label = strings.TrimSpace(s[p2+1:])
	} else {
		// non-ranged format: kind label
		df.Kind = strings.TrimSpace(s[0:space])
		df.Label = strings.TrimSpace(s[space+1:])
	}

	return df, nil
}

// a parsed data field
type DataField struct {
	// u8, ascii etc
	Kind string

	// ranged value if set. data field is type Kind[Length] or Kind[Start:End] (crc32)
	Range string

	// a slice type if true. data field is type Kind[]
	Slice bool

	// field label
	Label string
}

func (df *DataField) SingleUnitSize() uint64 {
	switch df.Kind {
	case "u8":
		return 1
	case "u16":
		return 2
	case "u32":
		return 4
	case "u64":
		return 8
	}
	panic("unreachable")
}

// presents the underlying type as it is known in the template format
func (df *DataField) PresentType() string {
	if df.Slice {
		return fmt.Sprintf("%s[]", df.Kind)
	}
	if df.Range != "" {
		return fmt.Sprintf("%s[%s]", df.Kind, df.Range)
	}
	return df.Kind
}

// returns true if unit is a single u8, u16, u32 or u64 that can have eq/bit field as child
func (df *DataField) IsPatternableUnit() bool {
	if df.Slice || df.Range != "" {
		return false
	}
	switch df.Kind {
	case "u8", "u16", "u32", "u64":
		return true
	}
	return false
}

// returns true if df.Range should be interpreted as a Kind[Start:End] range or false if its Kind[Length]
func (df *DataField) IsRangeUnit() bool {
	if df.Slice || df.Range == "" {
		return false
	}
	switch df.Kind {
	case "crc32":
		return true
	}
	return false
}

// reverse byte order in groups of `unitLength` to handle u16/u32 ordering
func ReverseBytes(b []byte, unitLength int) []byte {

	if len(b)%unitLength != 0 {
		log.Fatalf("invalid input '%v', length %d", b, unitLength)
	}

	res := make([]byte, len(b))
	for i := 0; i < len(b); i += unitLength {
		for j := 0; j < unitLength; j++ {
			res[i+unitLength-1-j] = b[i+j]
		}
	}

	return res
}
