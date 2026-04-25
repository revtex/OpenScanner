package shared

import (
	"fmt"
	"net/url"
	"strings"
)

// ContentDisposition builds an RFC 6266 Content-Disposition header
// value with both a legacy filename= token (ASCII-only, sanitised) and the
// percent-encoded filename*=UTF-8'' token for non-ASCII / unsafe characters.
// The caller supplies the disposition type (e.g. "inline" or "attachment").
func ContentDisposition(dispType, filename string) string {
	if filename == "" {
		filename = "file"
	}

	// ASCII fallback: replace characters that break quoted-string tokens.
	ascii := make([]byte, 0, len(filename))
	for i := 0; i < len(filename); i++ {
		b := filename[i]
		switch {
		case b == '"' || b == '\\' || b < 0x20 || b == 0x7f:
			ascii = append(ascii, '_')
		case b >= 0x80:
			ascii = append(ascii, '_')
		default:
			ascii = append(ascii, b)
		}
	}

	encoded := url.PathEscape(filename)
	// url.PathEscape does not encode a few characters that are invalid in
	// an ext-value token; patch them up for RFC 5987 compliance.
	encoded = strings.NewReplacer(
		"'", "%27",
		"(", "%28",
		")", "%29",
		"*", "%2A",
	).Replace(encoded)

	return fmt.Sprintf(`%s; filename="%s"; filename*=UTF-8''%s`, dispType, string(ascii), encoded)
}
