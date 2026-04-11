package dirwatch

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

	tok := TokensFromCall(callTime, "MySys", "MyTg", "Fire", "Alert", "5432", 12345, 851025000)

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

	tok := TokensFromCall(localTime, "", "", "", "", "", 0, 0)

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
			tok := TokensFromCall(time.Now(), "", "", "", "", "", 0, tc.freqHz)
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
	tok := TokensFromCall(time.Now(), "sys", "tg", "grp", "tag", "unit", 1, 851000000)
	if tok.TgAFS != "" {
		t.Errorf("TgAFS = %q, want empty string", tok.TgAFS)
	}
}
