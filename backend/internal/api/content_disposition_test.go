package api

import (
	"strings"
	"testing"
)

// TestContentDisposition verifies the RFC 6266 header builder for a range of
// filenames: plain ASCII, spaces, Unicode, quotes, control chars, empty,
// and RFC 5987 specials.
func TestContentDisposition(t *testing.T) {
	tests := []struct {
		name         string
		disposition  string
		filename     string
		wantASCII    string // expected filename="..." token value
		wantExtValue string // expected filename*=UTF-8''... token value
	}{
		{
			name:         "plain ASCII",
			disposition:  "attachment",
			filename:     "report.mp3",
			wantASCII:    "report.mp3",
			wantExtValue: "report.mp3",
		},
		{
			name:         "spaces",
			disposition:  "attachment",
			filename:     "my report.mp3",
			wantASCII:    "my report.mp3",
			wantExtValue: "my%20report.mp3",
		},
		{
			name:         "unicode cyrillic",
			disposition:  "inline",
			filename:     "отчёт.mp3",
			wantASCII:    "__________.mp3", // 5 cyrillic chars × 2 UTF-8 bytes each = 10 underscores
			wantExtValue: "%D0%BE%D1%82%D1%87%D1%91%D1%82.mp3",
		},
		{
			name:         "embedded quote",
			disposition:  "attachment",
			filename:     `bad"name.mp3`,
			wantASCII:    "bad_name.mp3",
			wantExtValue: "bad%22name.mp3",
		},
		{
			name:         "backslash and control chars",
			disposition:  "attachment",
			filename:     "a\\b\nc\td.mp3",
			wantASCII:    "a_b_c_d.mp3",
			wantExtValue: "a%5Cb%0Ac%09d.mp3",
		},
		{
			name:         "DEL (0x7f)",
			disposition:  "attachment",
			filename:     "pre\x7fpost.mp3",
			wantASCII:    "pre_post.mp3",
			wantExtValue: "pre%7Fpost.mp3",
		},
		{
			name:         "empty",
			disposition:  "attachment",
			filename:     "",
			wantASCII:    "file",
			wantExtValue: "file",
		},
		{
			name:         "rfc 5987 specials",
			disposition:  "attachment",
			filename:     "a(b)c*d'e.mp3",
			wantASCII:    "a(b)c*d'e.mp3",
			wantExtValue: "a%28b%29c%2Ad%27e.mp3",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := contentDisposition(tc.disposition, tc.filename)

			if !strings.HasPrefix(got, tc.disposition+"; ") {
				t.Errorf("header should begin with %q; got %q", tc.disposition+"; ", got)
			}

			wantFilename := `filename="` + tc.wantASCII + `"`
			if !strings.Contains(got, wantFilename) {
				t.Errorf("header missing %q; got %q", wantFilename, got)
			}

			wantExt := `filename*=UTF-8''` + tc.wantExtValue
			if !strings.Contains(got, wantExt) {
				t.Errorf("header missing %q; got %q", wantExt, got)
			}

			// Header must not contain raw CR/LF anywhere.
			if strings.ContainsAny(got, "\r\n") {
				t.Errorf("header contains raw CR/LF: %q", got)
			}

			// Must not contain a raw double-quote outside the quoted ASCII token.
			// There should be exactly two double-quotes (opening and closing of the
			// filename= token); any additional " would mean an unescaped quote leaked.
			if count := strings.Count(got, `"`); count != 2 {
				t.Errorf("header has %d double-quote chars, want 2: %q", count, got)
			}
		})
	}
}
