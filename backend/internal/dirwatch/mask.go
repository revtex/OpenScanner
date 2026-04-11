// Package dirwatch — meta-mask token expansion (#DATE, #TIME, #TGLBL, etc.).
package dirwatch

import (
	"fmt"
	"strings"
	"time"
)

// MaskTokens holds the values available for token substitution in a dir-mask.
type MaskTokens struct {
	Date     string // YYYYMMDD
	Time     string // HHMMSS
	ZTime    string // HHMMSS (zero-padded; same as Time)
	Group    string // talkgroup group label
	SysLabel string // system label
	Tag      string // talkgroup tag label
	TgAFS    string // AFS system identifier
	Unit     string // source unit ID string
	TgLabel  string // talkgroup label
	TgHz     string // frequency in Hz
	TgKHz    string // frequency in kHz
	TgMHz    string // frequency in MHz (X.XXX)
	TgID     string // talkgroup radio ID
}

// ExpandMask replaces all #TOKEN occurrences in mask with their corresponding
// values from t. Unrecognised tokens are left as-is.
func ExpandMask(mask string, t MaskTokens) string {
	r := strings.NewReplacer(
		"#DATE", t.Date,
		"#ZTIME", t.ZTime,
		"#TIME", t.Time,
		"#GROUP", t.Group,
		"#SYSLBL", t.SysLabel,
		"#TAG", t.Tag,
		"#TGAFS", t.TgAFS,
		"#UNIT", t.Unit,
		"#TGLBL", t.TgLabel,
		"#TGMHZ", t.TgMHz,
		"#TGKHZ", t.TgKHz,
		"#TGHZ", t.TgHz,
		"#TGID", t.TgID,
	)
	return r.Replace(mask)
}

// TokensFromCall builds MaskTokens from available call metadata.
func TokensFromCall(callTime time.Time, sysLabel, tgLabel, groupLabel, tagLabel, unit string, tgID, freqHz int64) MaskTokens {
	utc := callTime.UTC()
	return MaskTokens{
		Date:     utc.Format("20060102"),
		Time:     utc.Format("150405"),
		ZTime:    utc.Format("150405"),
		Group:    groupLabel,
		SysLabel: sysLabel,
		Tag:      tagLabel,
		TgAFS:    "",
		Unit:     unit,
		TgLabel:  tgLabel,
		TgHz:     fmt.Sprintf("%d", freqHz),
		TgKHz:    fmt.Sprintf("%d", freqHz/1000),
		TgMHz:    fmt.Sprintf("%.3f", float64(freqHz)/1_000_000),
		TgID:     fmt.Sprintf("%d", tgID),
	}
}
