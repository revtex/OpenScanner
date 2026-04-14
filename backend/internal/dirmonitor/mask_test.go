package dirmonitor

import (
	"testing"
	"time"
)

// TestExpandMask_IndividualTokens verifies that each of the 13 mask tokens is
// replaced correctly when used alone.
func TestExpandMask_IndividualTokens(t *testing.T) {
	cases := []struct {
		name   string
		mask   string
		tokens MaskTokens
		want   string
	}{
		{"DATE", "#DATE", MaskTokens{Date: "20240115"}, "20240115"},
		{"TIME", "#TIME", MaskTokens{Time: "143022"}, "143022"},
		{"ZTIME", "#ZTIME", MaskTokens{ZTime: "091500"}, "091500"},
		{"GROUP", "#GROUP", MaskTokens{Group: "Fire"}, "Fire"},
		{"SYSLBL", "#SYSLBL", MaskTokens{SysLabel: "MySystem"}, "MySystem"},
		{"TAG", "#TAG", MaskTokens{Tag: "Alert"}, "Alert"},
		{"TGAFS", "#TGAFS", MaskTokens{TgAFS: "AFS123"}, "AFS123"},
		{"UNIT", "#UNIT", MaskTokens{Unit: "5432"}, "5432"},
		{"TGLBL", "#TGLBL", MaskTokens{TgLabel: "Dispatch"}, "Dispatch"},
		{"TGHZ", "#TGHZ", MaskTokens{TgHz: "851025000"}, "851025000"},
		{"TGKHZ", "#TGKHZ", MaskTokens{TgKHz: "851025"}, "851025"},
		{"TGMHZ", "#TGMHZ", MaskTokens{TgMHz: "851.025"}, "851.025"},
		{"TGID", "#TGID", MaskTokens{TgID: "12345"}, "12345"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ExpandMask(tc.mask, tc.tokens)
			if got != tc.want {
				t.Errorf("ExpandMask(%q) = %q, want %q", tc.mask, got, tc.want)
			}
		})
	}
}

// TestExpandMask_MultipleTokens verifies that several tokens in one mask are
// all expanded in a single pass.
func TestExpandMask_MultipleTokens(t *testing.T) {
	tokens := MaskTokens{
		Date:    "20240115",
		Time:    "143022",
		TgLabel: "Dispatch",
		Unit:    "5432",
		TgMHz:   "851.025",
	}
	cases := []struct {
		name string
		mask string
		want string
	}{
		{
			name: "date/time path",
			mask: "#DATE/#TIME",
			want: "20240115/143022",
		},
		{
			name: "all five tokens",
			mask: "#DATE/#TIME/#TGLBL/#UNIT/#TGMHZ",
			want: "20240115/143022/Dispatch/5432/851.025",
		},
		{
			name: "repeated token",
			mask: "#DATE-#DATE",
			want: "20240115-20240115",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ExpandMask(tc.mask, tokens)
			if got != tc.want {
				t.Errorf("ExpandMask(%q) = %q, want %q", tc.mask, got, tc.want)
			}
		})
	}
}

// TestExpandMask_UnknownTokenLeftAsIs ensures unrecognised #TOKENS are
// preserved exactly as written.
func TestExpandMask_UnknownTokenLeftAsIs(t *testing.T) {
	cases := []struct {
		mask string
	}{
		{"#UNKNOWN"},
		{"#UNKNOWN/path"},
		{"prefix/#NOTAKEY/suffix"},
	}

	for _, tc := range cases {
		t.Run(tc.mask, func(t *testing.T) {
			got := ExpandMask(tc.mask, MaskTokens{Date: "20240101"})
			if got != tc.mask {
				t.Errorf("ExpandMask(%q) = %q, want unchanged %q", tc.mask, got, tc.mask)
			}
		})
	}
}

// TestExpandMask_PrefixMatchBehaviour documents that strings.NewReplacer
// expands the longest known prefix it encounters. "#DATEX" contains "#DATE"
// as a prefix so it becomes "<Date>X", not the literal string "#DATEX".
func TestExpandMask_PrefixMatchBehaviour(t *testing.T) {
	got := ExpandMask("#DATEX", MaskTokens{Date: "20240101"})
	want := "20240101X"
	if got != want {
		t.Errorf("ExpandMask(\"#DATEX\") = %q, want %q", got, want)
	}
}

// TestExpandMask_EmptyMask verifies that an empty mask returns an empty string.
func TestExpandMask_EmptyMask(t *testing.T) {
	got := ExpandMask("", MaskTokens{Date: "20240115"})
	if got != "" {
		t.Errorf("ExpandMask(\"\") = %q, want empty string", got)
	}
}

// TestExpandMask_NoTokensInMask verifies that a mask without any tokens is
// returned unchanged.
func TestExpandMask_NoTokensInMask(t *testing.T) {
	mask := "static/path/to/recordings"
	got := ExpandMask(mask, MaskTokens{TgLabel: "Dispatch"})
	if got != mask {
		t.Errorf("ExpandMask(%q) = %q, want %q", mask, got, mask)
	}
}

// TestExpandMask_FreqTokensDoNotOverlap verifies that #TGKHZ, #TGMHZ, and
// #TGHZ are each replaced independently without corrupting one another.
func TestExpandMask_FreqTokensDoNotOverlap(t *testing.T) {
	tokens := MaskTokens{
		TgHz:  "851025000",
		TgKHz: "851025",
		TgMHz: "851.025",
	}
	mask := "#TGMHZ/#TGKHZ/#TGHZ"
	want := "851.025/851025/851025000"
	got := ExpandMask(mask, tokens)
	if got != want {
		t.Errorf("ExpandMask(%q) = %q, want %q", mask, got, want)
	}
}

// TestTokensFromCall_FieldValues verifies that each field in the returned
// MaskTokens matches the expected formatted value.
func TestTokensFromCall_FieldValues(t *testing.T) {
	// 2024-01-15 14:30:22 UTC
	callTime := time.Date(2024, 1, 15, 14, 30, 22, 0, time.UTC)

	tok := TokensFromCall(callTime, "MySys", 42, "MyTg", "Fire", "Alert", "5432", 12345, 851025000)

	cases := []struct {
		name string
		got  string
		want string
	}{
		{"Date YYYYMMDD", tok.Date, "20240115"},
		{"Time HHMMSS", tok.Time, "143022"},
		{"ZTime HHMMSS", tok.ZTime, "143022"},
		{"SysLabel", tok.SysLabel, "MySys"},
		{"TgLabel", tok.TgLabel, "MyTg"},
		{"Group", tok.Group, "Fire"},
		{"Tag", tok.Tag, "Alert"},
		{"Unit", tok.Unit, "5432"},
		{"TgID", tok.TgID, "12345"},
		{"TgHz", tok.TgHz, "851025000"},
		{"TgKHz", tok.TgKHz, "851025"},
		{"TgMHz", tok.TgMHz, "851.025"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.got != tc.want {
				t.Errorf("TokensFromCall %s = %q, want %q", tc.name, tc.got, tc.want)
			}
		})
	}
}

// TestTokensFromCall_UTC verifies that date/time values are always expressed
// in UTC even when the input time carries a non-UTC location.
func TestTokensFromCall_UTC(t *testing.T) {
	// 2024-01-15 09:30:22 EST  →  2024-01-15 14:30:22 UTC
	est := time.FixedZone("EST", -5*60*60)
	localTime := time.Date(2024, 1, 15, 9, 30, 22, 0, est)

	tok := TokensFromCall(localTime, "", 0, "", "", "", "", 0, 0)

	if tok.Date != "20240115" {
		t.Errorf("Date = %q, want %q (expected UTC date)", tok.Date, "20240115")
	}
	if tok.Time != "143022" {
		t.Errorf("Time = %q, want %q (expected UTC time)", tok.Time, "143022")
	}
}

// TestTokensFromCall_FrequencyFormats verifies the three frequency format
// variants (Hz, kHz, MHz) for a range of inputs.
func TestTokensFromCall_FrequencyFormats(t *testing.T) {
	cases := []struct {
		freqHz  int64
		wantHz  string
		wantKHz string
		wantMHz string
	}{
		{851025000, "851025000", "851025", "851.025"},
		{155000000, "155000000", "155000", "155.000"},
		{0, "0", "0", "0.000"},
		{460100000, "460100000", "460100", "460.100"},
		{1234567890, "1234567890", "1234567", "1234.568"},
	}

	for _, tc := range cases {
		t.Run(tc.wantMHz, func(t *testing.T) {
			tok := TokensFromCall(time.Now(), "", 0, "", "", "", "", 0, tc.freqHz)
			if tok.TgHz != tc.wantHz {
				t.Errorf("freq %d: TgHz = %q, want %q", tc.freqHz, tok.TgHz, tc.wantHz)
			}
			if tok.TgKHz != tc.wantKHz {
				t.Errorf("freq %d: TgKHz = %q, want %q", tc.freqHz, tok.TgKHz, tc.wantKHz)
			}
			if tok.TgMHz != tc.wantMHz {
				t.Errorf("freq %d: TgMHz = %q, want %q", tc.freqHz, tok.TgMHz, tc.wantMHz)
			}
		})
	}
}

// TestTokensFromCall_TgAFSEmpty verifies that TgAFS is always an empty string
// (not yet implemented in TokensFromCall).
func TestTokensFromCall_TgAFSEmpty(t *testing.T) {
	tok := TokensFromCall(time.Now(), "sys", 1, "tg", "grp", "tag", "unit", 1, 851000000)
	if tok.TgAFS != "" {
		t.Errorf("TgAFS = %q, want empty string", tok.TgAFS)
	}
}

// ── ParseMask tests ──────────────────────────────────────────────────────────

// TestParseMask_ProScanWithParens is the exact scenario from the bug report:
// a ProScan mask with parentheses in the talkgroup label.
func TestParseMask_ProScanWithParens(t *testing.T) {
	mask := "#DATE_#TIME_#GROUP_#TGLBL_#TG"
	filename := "2025-08-17_12-15-16_St. Johns County Fire Rescue_A1 Primary (Dispatch)_10000"

	values, ok := ParseMask(mask, filename)
	if !ok {
		t.Fatal("ParseMask returned false; expected match")
	}

	want := map[string]string{
		"#DATE":  "2025-08-17",
		"#TIME":  "12-15-16",
		"#GROUP": "St. Johns County Fire Rescue",
		"#TGLBL": "A1 Primary (Dispatch)",
		"#TG":    "10000",
	}
	for k, v := range want {
		if values[k] != v {
			t.Errorf("token %s = %q, want %q", k, values[k], v)
		}
	}
}

// TestParseMask_NumericOnly tests a mask with only numeric tokens.
func TestParseMask_NumericOnly(t *testing.T) {
	mask := "#SYS-#TG"
	filename := "42-10000"

	values, ok := ParseMask(mask, filename)
	if !ok {
		t.Fatal("ParseMask returned false")
	}
	if values["#SYS"] != "42" {
		t.Errorf("#SYS = %q, want %q", values["#SYS"], "42")
	}
	if values["#TG"] != "10000" {
		t.Errorf("#TG = %q, want %q", values["#TG"], "10000")
	}
}

// TestParseMask_NoTokens returns false for a mask without tokens.
func TestParseMask_NoTokens(t *testing.T) {
	_, ok := ParseMask("static_mask", "anything")
	if ok {
		t.Error("ParseMask should return false for mask without tokens")
	}
}

// TestParseMask_NoMatch returns false when the filename doesn't match.
func TestParseMask_NoMatch(t *testing.T) {
	mask := "#DATE_#TIME_#TG"
	filename := "no-underscores-here"

	_, ok := ParseMask(mask, filename)
	if ok {
		t.Error("ParseMask should return false for non-matching filename")
	}
}

// TestParseMask_FrequencyTokens tests MHz/kHz/Hz token extraction.
func TestParseMask_FrequencyTokens(t *testing.T) {
	mask := "#DATE_#TGMHZ_#TG"
	filename := "20250817_851.025_10000"

	values, ok := ParseMask(mask, filename)
	if !ok {
		t.Fatal("ParseMask returned false")
	}
	if values["#TGMHZ"] != "851.025" {
		t.Errorf("#TGMHZ = %q, want %q", values["#TGMHZ"], "851.025")
	}
}

// TestParseMask_TGID verifies #TGID is extracted separately from #TG.
func TestParseMask_TGID(t *testing.T) {
	mask := "#DATE_#TGID"
	filename := "20250817_99999"

	values, ok := ParseMask(mask, filename)
	if !ok {
		t.Fatal("ParseMask returned false")
	}
	if values["#TGID"] != "99999" {
		t.Errorf("#TGID = %q, want %q", values["#TGID"], "99999")
	}
}

// TestParseMask_CompactDatetime tests masks with compact YYYYMMDD/HHMMSS.
func TestParseMask_CompactDatetime(t *testing.T) {
	mask := "#DATE_#TIME_#TG"
	filename := "20250817_121516_10000"

	values, ok := ParseMask(mask, filename)
	if !ok {
		t.Fatal("ParseMask returned false")
	}
	if values["#DATE"] != "20250817" {
		t.Errorf("#DATE = %q, want %q", values["#DATE"], "20250817")
	}
	if values["#TIME"] != "121516" {
		t.Errorf("#TIME = %q, want %q", values["#TIME"], "121516")
	}
}

// TestParseMask_SysLabel tests the #SYSLBL token.
func TestParseMask_SysLabel(t *testing.T) {
	mask := "#SYSLBL_#TG"
	filename := "My System_10000"

	values, ok := ParseMask(mask, filename)
	if !ok {
		t.Fatal("ParseMask returned false")
	}
	if values["#SYSLBL"] != "My System" {
		t.Errorf("#SYSLBL = %q, want %q", values["#SYSLBL"], "My System")
	}
}

// ── ApplyMaskValues tests ────────────────────────────────────────────────────

// TestApplyMaskValues_FillsZeroFields verifies that mask values fill in
// zero-valued fields.
func TestApplyMaskValues_FillsZeroFields(t *testing.T) {
	call := &ParsedCall{
		AudioFilePath: "/tmp/test.wav",
		DateTime:      time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	values := map[string]string{
		"#TG":    "10000",
		"#SYS":   "42",
		"#TGLBL": "A1 Primary (Dispatch)",
		"#UNIT":  "5432",
	}
	ApplyMaskValues(call, values)

	if call.TalkgroupID != 10000 {
		t.Errorf("TalkgroupID = %d, want 10000", call.TalkgroupID)
	}
	if call.SystemID != 42 {
		t.Errorf("SystemID = %d, want 42", call.SystemID)
	}
	if call.TalkgroupTitle != "A1 Primary (Dispatch)" {
		t.Errorf("TalkgroupTitle = %q, want %q", call.TalkgroupTitle, "A1 Primary (Dispatch)")
	}
	if call.Source != 5432 {
		t.Errorf("Source = %d, want 5432", call.Source)
	}
}

// TestApplyMaskValues_DoesNotOverwriteNonZero verifies that non-zero fields
// set by the parser or config overrides are preserved.
func TestApplyMaskValues_DoesNotOverwriteNonZero(t *testing.T) {
	call := &ParsedCall{
		AudioFilePath:  "/tmp/test.wav",
		TalkgroupID:    999, // already set by config override
		SystemID:       5,   // already set
		TalkgroupTitle: "Existing",
	}
	values := map[string]string{
		"#TG":    "10000",
		"#SYS":   "42",
		"#TGLBL": "From Mask",
	}
	ApplyMaskValues(call, values)

	if call.TalkgroupID != 999 {
		t.Errorf("TalkgroupID = %d, want 999 (should not be overwritten)", call.TalkgroupID)
	}
	if call.SystemID != 5 {
		t.Errorf("SystemID = %d, want 5 (should not be overwritten)", call.SystemID)
	}
	if call.TalkgroupTitle != "Existing" {
		t.Errorf("TalkgroupTitle = %q, want %q (should not be overwritten)", call.TalkgroupTitle, "Existing")
	}
}

// TestApplyMaskValues_DateTime verifies that mask-extracted date/time always
// replaces the parsed DateTime (which is typically file ModTime).
func TestApplyMaskValues_DateTime(t *testing.T) {
	modTime := time.Date(2025, 8, 17, 16, 0, 0, 0, time.UTC)
	call := &ParsedCall{
		AudioFilePath: "/tmp/test.wav",
		DateTime:      modTime,
	}
	values := map[string]string{
		"#DATE": "2025-08-17",
		"#TIME": "12-15-16",
	}
	ApplyMaskValues(call, values)

	if call.DateTime.Equal(modTime) {
		t.Error("DateTime should have been replaced by mask-extracted value")
	}
	// Verify the masked date/time was parsed (in local TZ, so compare components).
	if call.DateTime.Year() != 2025 || call.DateTime.Month() != 8 || call.DateTime.Day() != 17 {
		t.Errorf("DateTime date = %v, want 2025-08-17", call.DateTime)
	}
	if call.DateTime.Hour() != 12 || call.DateTime.Minute() != 15 || call.DateTime.Second() != 16 {
		t.Errorf("DateTime time = %v, want 12:15:16", call.DateTime)
	}
}

// TestApplyMaskValues_FrequencyMHz verifies MHz frequency extraction.
func TestApplyMaskValues_FrequencyMHz(t *testing.T) {
	call := &ParsedCall{AudioFilePath: "/tmp/test.wav"}
	ApplyMaskValues(call, map[string]string{"#TGMHZ": "851.025"})
	if call.Frequency != 851025000 {
		t.Errorf("Frequency = %d, want 851025000", call.Frequency)
	}
}

// TestApplyMaskValues_TGID verifies #TGID takes precedence over #TG.
func TestApplyMaskValues_TGID(t *testing.T) {
	call := &ParsedCall{AudioFilePath: "/tmp/test.wav"}
	ApplyMaskValues(call, map[string]string{
		"#TGID": "12345",
		"#TG":   "99999",
	})
	if call.TalkgroupID != 12345 {
		t.Errorf("TalkgroupID = %d, want 12345 (#TGID should take precedence)", call.TalkgroupID)
	}
}

// TestParseMaskDateTime verifies various date/time format combinations.
func TestParseMaskDateTime(t *testing.T) {
	cases := []struct {
		name     string
		dateStr  string
		timeStr  string
		wantOK   bool
		wantYear int
		wantMon  time.Month
		wantDay  int
		wantHour int
		wantMin  int
		wantSec  int
	}{
		{"dashed", "2025-08-17", "12-15-16", true, 2025, 8, 17, 12, 15, 16},
		{"compact", "20250817", "121516", true, 2025, 8, 17, 12, 15, 16},
		{"colons", "2025-08-17", "12:15:16", true, 2025, 8, 17, 12, 15, 16},
		{"short_date", "2025", "121516", false, 0, 0, 0, 0, 0, 0},
		{"short_time", "20250817", "12", true, 2025, 8, 17, 12, 0, 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := parseMaskDateTime(tc.dateStr, tc.timeStr)
			if ok != tc.wantOK {
				t.Fatalf("parseMaskDateTime(%q, %q) ok = %v, want %v", tc.dateStr, tc.timeStr, ok, tc.wantOK)
			}
			if !ok {
				return
			}
			if got.Year() != tc.wantYear || got.Month() != tc.wantMon || got.Day() != tc.wantDay {
				t.Errorf("date = %d-%02d-%02d, want %d-%02d-%02d",
					got.Year(), got.Month(), got.Day(), tc.wantYear, tc.wantMon, tc.wantDay)
			}
			if got.Hour() != tc.wantHour || got.Minute() != tc.wantMin || got.Second() != tc.wantSec {
				t.Errorf("time = %02d:%02d:%02d, want %02d:%02d:%02d",
					got.Hour(), got.Minute(), got.Second(), tc.wantHour, tc.wantMin, tc.wantSec)
			}
		})
	}
}
