// Package dirmonitor — meta-mask token expansion (#DATE, #TIME, #TGLBL, etc.)
// and reverse mask parsing (filename → metadata extraction).
package dirmonitor

import (
	"fmt"
	"regexp"
	"strconv"
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
	Sys      string // system radio ID
	Hz       string // generic frequency in Hz (non-talkgroup)
	KHz      string // generic frequency in kHz
	MHz      string // generic frequency in MHz
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
		"#SYS", t.Sys,
		"#MHZ", t.MHz,
		"#KHZ", t.KHz,
		"#HZ", t.Hz,
		"#TG", t.TgID,
	)
	return r.Replace(mask)
}

// TokensFromCall builds MaskTokens from available call metadata.
func TokensFromCall(callTime time.Time, sysLabel string, sysID int64, tgLabel, groupLabel, tagLabel, unit string, tgID, freqHz int64) MaskTokens {
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
		Sys:      fmt.Sprintf("%d", sysID),
		Hz:       fmt.Sprintf("%d", freqHz),
		KHz:      fmt.Sprintf("%d", freqHz/1000),
		MHz:      fmt.Sprintf("%.3f", float64(freqHz)/1_000_000),
	}
}

// ── Reverse mask parsing (filename → metadata) ───────────────────────────────

// reToken matches a mask token: # followed by one or more uppercase letters.
var reToken = regexp.MustCompile(`#[A-Z]+`)

// tokenCapturePattern maps known mask tokens to regex sub-patterns used when
// building a regex from a mask for filename matching.
//
// Text tokens use (.+?) (non-greedy) so that literal separators between tokens
// act as anchors. Numeric tokens use (\d+). Date/time tokens use (\S+?) to
// match compact values that may contain dashes or colons but not spaces.
var tokenCapturePattern = map[string]string{
	"#DATE":   `(\S+?)`,
	"#ZTIME":  `(\S+?)`,
	"#TIME":   `(\S+?)`,
	"#GROUP":  `(.+?)`,
	"#SYSLBL": `(.+?)`,
	"#TAG":    `(.+?)`,
	"#TGAFS":  `(.+?)`,
	"#UNIT":   `(\d+)`,
	"#TGLBL":  `(.+?)`,
	"#TGMHZ":  `([\d.]+)`,
	"#TGKHZ":  `(\d+)`,
	"#TGHZ":   `(\d+)`,
	"#TGID":   `(\d+)`,
	"#SYS":    `(\d+)`,
	"#MHZ":    `([\d.]+)`,
	"#KHZ":    `(\d+)`,
	"#HZ":     `(\d+)`,
	"#TG":     `(\d+)`,
}

// ParseMask converts a mask pattern into a regex, matches it against the
// given filename (without extension), and returns a map of token→value for
// each matched token. Returns nil, false if the mask has no tokens or the
// filename doesn't match.
//
// Example:
//
//	mask = "#DATE_#TIME_#GROUP_#TGLBL_#TG"
//	filename = "2025-08-17_12-15-16_St. Johns County Fire Rescue_A1 Primary (Dispatch)_10000"
//	→ {"#DATE":"2025-08-17", "#TIME":"12-15-16", "#GROUP":"St. Johns County Fire Rescue",
//	   "#TGLBL":"A1 Primary (Dispatch)", "#TG":"10000"}, true
func ParseMask(mask, filename string) (map[string]string, bool) {
	locs := reToken.FindAllStringIndex(mask, -1)
	if len(locs) == 0 {
		return nil, false
	}

	var pattern strings.Builder
	var names []string
	pattern.WriteString("^")
	lastEnd := 0

	for _, loc := range locs {
		// Escape literal text between tokens (separators like _ or -).
		if loc[0] > lastEnd {
			pattern.WriteString(regexp.QuoteMeta(mask[lastEnd:loc[0]]))
		}
		token := mask[loc[0]:loc[1]]
		if p, ok := tokenCapturePattern[token]; ok {
			pattern.WriteString(p)
			names = append(names, token)
		} else {
			// Unknown token — treat as literal.
			pattern.WriteString(regexp.QuoteMeta(token))
		}
		lastEnd = loc[1]
	}

	if lastEnd < len(mask) {
		pattern.WriteString(regexp.QuoteMeta(mask[lastEnd:]))
	}
	pattern.WriteString("$")

	re, err := regexp.Compile(pattern.String())
	if err != nil {
		return nil, false
	}

	subs := re.FindStringSubmatch(filename)
	if subs == nil {
		return nil, false
	}

	result := make(map[string]string, len(names))
	for i, name := range names {
		if i+1 < len(subs) {
			result[name] = subs[i+1]
		}
	}
	return result, true
}

// ApplyMaskValues fills zero-valued fields in call with values extracted from
// mask token matching. Non-zero fields (already set by the parser or config
// overrides) are never overwritten — except DateTime, which is replaced when
// the mask provides both #DATE and #TIME because the mask timestamp is more
// accurate than file ModTime.
func ApplyMaskValues(call *ParsedCall, values map[string]string) {
	// Talkgroup ID from #TGID or #TG.
	if call.TalkgroupID == 0 {
		for _, tok := range []string{"#TGID", "#TG"} {
			if s, ok := values[tok]; ok {
				if v, err := strconv.ParseInt(s, 10, 64); err == nil && v > 0 {
					call.TalkgroupID = v
					break
				}
			}
		}
	}

	// System ID from #SYS.
	if call.SystemID == 0 {
		if s, ok := values["#SYS"]; ok {
			if v, err := strconv.ParseInt(s, 10, 64); err == nil && v > 0 {
				call.SystemID = v
			}
		}
	}

	// System label from #SYSLBL.
	if call.SystemLabel == "" {
		if s, ok := values["#SYSLBL"]; ok {
			call.SystemLabel = s
		}
	}

	// Talkgroup title from #TGLBL.
	if call.TalkgroupTitle == "" {
		if s, ok := values["#TGLBL"]; ok {
			call.TalkgroupTitle = s
		}
	}

	// Talkgroup group label from #GROUP.
	if call.TalkgroupGroup == "" {
		if s, ok := values["#GROUP"]; ok && len(s) > 0 && s != "-" {
			call.TalkgroupGroup = s
		}
	}

	// Talkgroup tag label from #TAG.
	if call.TalkgroupTag == "" {
		if s, ok := values["#TAG"]; ok && len(s) > 0 && s != "-" {
			call.TalkgroupTag = s
		}
	}

	// Talkgroup ID from #TGAFS (AFS format: DD-DDD where a<<7|b<<3|c).
	if call.TalkgroupID == 0 {
		if s, ok := values["#TGAFS"]; ok && len(s) == 6 && s[2] == '-' {
			a, errA := strconv.Atoi(s[:2])
			b, errB := strconv.Atoi(s[3:5])
			c, errC := strconv.Atoi(s[5:])
			if errA == nil && errB == nil && errC == nil {
				call.TalkgroupID = int64(a<<7 | b<<3 | c)
			}
		}
	}

	// Source unit from #UNIT.
	if call.Source == 0 {
		if s, ok := values["#UNIT"]; ok {
			if v, err := strconv.ParseInt(s, 10, 64); err == nil && v > 0 {
				call.Source = v
			}
		}
	}

	// Frequency (and optionally TalkgroupID) from Hz/kHz/MHz tokens.
	// #TGHZ, #TGKHZ, #TGMHZ set both Frequency AND TalkgroupID (matching
	// rdio-scanner behaviour for conventional systems where the frequency
	// identifies the talkgroup). #HZ, #KHZ, #MHZ set Frequency only.
	if call.Frequency == 0 {
		type ft struct {
			token  string
			mult   float64
			setsTG bool
		}
		for _, f := range []ft{
			{"#HZ", 1, false}, {"#TGHZ", 1, true},
			{"#KHZ", 1000, false}, {"#TGKHZ", 1000, true},
			{"#MHZ", 1_000_000, false}, {"#TGMHZ", 1_000_000, true},
		} {
			if s, ok := values[f.token]; ok {
				var hz float64
				if f.mult == 1 {
					if v, err := strconv.ParseInt(s, 10, 64); err == nil && v > 0 {
						hz = float64(v)
					}
				} else {
					if v, err := strconv.ParseFloat(s, 64); err == nil && v > 0 {
						hz = v * f.mult
					}
				}
				if hz > 0 {
					call.Frequency = int64(hz)
					if f.setsTG && call.TalkgroupID == 0 {
						call.TalkgroupID = int64(hz / 1000)
					}
					break
				}
			}
		}
	}

	// Date + Time: prefer mask-extracted timestamp over file ModTime.
	dateStr, hasDate := values["#DATE"]
	timeStr, hasTime := values["#TIME"]
	if !hasTime {
		timeStr, hasTime = values["#ZTIME"]
	}
	if hasDate && hasTime {
		if t, ok := parseMaskDateTime(dateStr, timeStr); ok {
			call.DateTime = t
		}
	}
}

// parseMaskDateTime parses date and time strings extracted from a mask match.
// Handles common formats by stripping separators (-, /, :, .) and parsing
// as YYYYMMDDHHMMSS. Returns the parsed time in local timezone.
func parseMaskDateTime(dateStr, timeStr string) (time.Time, bool) {
	strip := strings.NewReplacer("-", "", "/", "", ":", "", ".", "")
	d := strip.Replace(dateStr)
	t := strip.Replace(timeStr)

	if len(d) != 8 {
		return time.Time{}, false
	}
	// Pad short time values (e.g. HHMM → HHMM00).
	for len(t) < 6 {
		t += "0"
	}

	parsed, err := time.ParseInLocation("20060102150405", d+t[:6], time.Now().Location())
	if err != nil {
		return time.Time{}, false
	}
	return parsed, true
}
