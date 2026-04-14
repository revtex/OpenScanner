// Package dirmonitor — per-recorder-type file parsers.
package dirmonitor

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/openscanner/openscanner/internal/db"
)

// ParsedCall is the normalised metadata extracted by a recorder-specific parser.
type ParsedCall struct {
	AudioFilePath  string    // absolute path to audio file on disk
	SidecarPath    string    // absolute path to companion metadata file (e.g. .json); "" if none
	DateTime       time.Time // call timestamp
	SystemID       int64     // radio system ID (0 = unknown)
	SystemLabel    string    // radio system label (used when SystemID is unknown)
	TalkgroupID    int64     // radio talkgroup ID (0 = unknown)
	TalkgroupTitle string    // talkgroup label / alpha tag from metadata
	TalkgroupName  string    // talkgroup description / name; "" if unknown
	TalkgroupTag   string    // talkgroup tag category label (e.g. "Law Dispatch"); "" if unknown
	TalkgroupGroup string    // talkgroup group label (e.g. "Police"); "" if unknown
	Frequency      int64     // Hz; 0 if unknown
	Duration       int64     // ms; 0 if unknown
	Source         int64     // source unit ID; 0 if unknown
	SourcesJSON    string    // JSON array string; "" if unknown
	FreqsJSON      string    // JSON array string; "" if unknown
	PatchesJSON    string    // JSON array string; "" if unknown
	Site           string    // receiver site name; "" if unknown
	Channel        string    // channel identifier; "" if unknown
	Decoder        string    // decoder type (e.g. "P25 Phase 1"); "" if unknown
}

// ParseFunc parses a newly detected file for a given dirmonitor config.
// It may return (nil, nil) when the file should be ignored (e.g. a sidecar
// whose matching audio has not yet arrived).
type ParseFunc func(dw db.Dirmonitor, triggeredPath string) (*ParsedCall, error)

// audioExtensions is the set of recognised audio file extensions (lower-case).
var audioExtensions = map[string]bool{
	".mp3":  true,
	".wav":  true,
	".m4a":  true,
	".aac":  true,
	".ogg":  true,
	".flac": true,
	".opus": true,
}

// isAudioFile returns true when path has a recognised audio extension.
func isAudioFile(path string) bool {
	return audioExtensions[strings.ToLower(filepath.Ext(path))]
}

// parserForType returns the ParseFunc for the given recorder type string.
// Unrecognised types fall back to parseGeneric.
func parserForType(recorderType string) ParseFunc {
	switch strings.ToLower(recorderType) {
	case "trunk-recorder":
		return parseTrunkRecorder
	case "sdrtrunk", "sdr-trunk":
		return parseSDRTrunk
	case "rtlsdr-airband":
		return parseRTLSDRAirband
	case "dsdplus":
		return parseDSDPlus
	case "proscan":
		return parseProScan
	default:
		return parseGeneric
	}
}

// ── Trunk Recorder ──────────────────────────────────────────────────────────

// trunkRecorderSidecar mirrors the JSON sidecar written by Trunk Recorder.
type trunkRecorderSidecar struct {
	StartTime            int64           `json:"start_time"`
	CallLength           float64         `json:"call_length"`
	Freq                 int64           `json:"freq"`
	Talkgroup            int64           `json:"talkgroup"`
	ShortName            string          `json:"short_name"`
	TalkgroupTag         string          `json:"talkgroup_tag"`
	TalkgroupAlphaTag    string          `json:"talkgroup_alpha_tag"`
	TalkgroupDescription string          `json:"talkgroup_description"`
	TalkgroupGroup       string          `json:"talkgroup_group"`
	SourceNum            int64           `json:"source_num"`
	SrcList              json.RawMessage `json:"srcList"`
	FreqList             json.RawMessage `json:"freqList"`
	PatchedTalkgroups    json.RawMessage `json:"patched_talkgroups"`
	// sys_num is not present in current Trunk Recorder JSON output,
	// but we keep it for backward compatibility with older versions.
	SysNum int64 `json:"sys_num"`
}

// parseTrunkRecorder handles both JSON sidecars and audio files.
//
//   - JSON triggered  →  parse sidecar, find matching audio; return nil if audio absent.
//   - Audio triggered →  look for .json sidecar; return nil if sidecar absent.
func parseTrunkRecorder(dw db.Dirmonitor, triggeredPath string) (*ParsedCall, error) {
	ext := strings.ToLower(filepath.Ext(triggeredPath))
	stem := triggeredPath[:len(triggeredPath)-len(filepath.Ext(triggeredPath))]

	var jsonPath, audioPath string
	switch {
	case ext == ".json":
		jsonPath = triggeredPath
		audioPath = findAudioSibling(stem)
		if audioPath == "" {
			// Audio file has not arrived yet — caller will retry when it does.
			return nil, nil
		}
	case isAudioFile(triggeredPath):
		audioPath = triggeredPath
		jsonPath = stem + ".json"
	default:
		return nil, nil
	}

	// SECURITY: limit sidecar reads to 1 MiB to prevent OOM from crafted files.
	const maxSidecarBytes = 1 << 20 // 1 MiB
	f, err := os.Open(jsonPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Sidecar not yet written — caller will retry when it arrives.
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxSidecarBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxSidecarBytes {
		return nil, fmt.Errorf("sidecar file too large (max %d bytes)", maxSidecarBytes)
	}
	var sc trunkRecorderSidecar
	if err := json.Unmarshal(data, &sc); err != nil {
		return nil, err
	}

	// talkgroup_alpha_tag is the human-readable label (e.g. "Metro PD Dispatch").
	// talkgroup_tag is the category (e.g. "Law Dispatch") — resolves to tag_id.
	// Fall back to talkgroup_tag for label when alpha_tag is absent (old sidecars).
	tgLabel := sc.TalkgroupAlphaTag
	if tgLabel == "" {
		tgLabel = sc.TalkgroupTag
	}

	return &ParsedCall{
		AudioFilePath:  audioPath,
		SidecarPath:    jsonPath,
		DateTime:       time.Unix(sc.StartTime, 0),
		SystemID:       sc.SysNum,
		SystemLabel:    sc.ShortName,
		TalkgroupID:    sc.Talkgroup,
		TalkgroupTitle: tgLabel,
		TalkgroupName:  sc.TalkgroupDescription,
		TalkgroupTag:   sc.TalkgroupTag,
		TalkgroupGroup: sc.TalkgroupGroup,
		Frequency:      sc.Freq,
		Duration:       int64(sc.CallLength * 1000),
		Source:         sc.SourceNum,
		SourcesJSON:    rawOrEmpty(sc.SrcList),
		FreqsJSON:      rawOrEmpty(sc.FreqList),
		PatchesJSON:    rawOrEmpty(sc.PatchedTalkgroups),
	}, nil
}

// findAudioSibling looks for an audio file sharing base with stem.
func findAudioSibling(stem string) string {
	for ext := range audioExtensions {
		p := stem + ext
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// rawOrEmpty converts a json.RawMessage to a string, returning "" for null/empty.
func rawOrEmpty(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	return string(raw)
}

// ── SDRTrunk ─────────────────────────────────────────────────────────────────

// Compiled regexes for SDR Trunk ID3 tag parsing.
var (
	reSDRArtist  = regexp.MustCompile(`^([0-9]+) ?(.*)$`)
	reSDRDate    = regexp.MustCompile(`Date:([^;]+);`)
	reSDRFreq    = regexp.MustCompile(`Frequency:([0-9]+);`)
	reSDRSystem  = regexp.MustCompile(`System:([^;]+);`)
	reSDRSite    = regexp.MustCompile(`Site:([^;]+);`)
	reSDRChannel = regexp.MustCompile(`Channel:([^;]+);`)
	reSDRDecoder = regexp.MustCompile(`Decoder:([^;]+);`)
	reSDRTgID    = regexp.MustCompile(`([0-9]+)`)
)

// parseSDRTrunk parses SDR Trunk recordings. It first attempts to read
// metadata from MP3 ID3v2 tags (Artist, Comment, Title). If tag reading
// fails or the file is not an MP3, it falls back to filename pattern
// parsing: <systemID>_<talkgroupID>_<unixTs>.<ext>
func parseSDRTrunk(dw db.Dirmonitor, triggeredPath string) (*ParsedCall, error) {
	if !isAudioFile(triggeredPath) {
		return nil, nil
	}

	var sysID, tgID, freq, source int64
	var systemLabel, tgTitle string
	var site, channel, decoder string
	var dt time.Time

	// Try ID3 tag reading first.
	if tagsParsed := parseSDRTrunkTags(triggeredPath, &systemLabel, &tgID, &freq, &source, &dt, &tgTitle, &site, &channel, &decoder); !tagsParsed {
		// Fall back to filename parsing: <systemID>_<talkgroupID>_<unixTs>.<ext>
		name := filepath.Base(triggeredPath)
		stem := strings.TrimSuffix(name, filepath.Ext(name))
		parts := strings.SplitN(stem, "_", 3)
		if len(parts) == 3 {
			sysID, _ = strconv.ParseInt(parts[0], 10, 64)
			tgID, _ = strconv.ParseInt(parts[1], 10, 64)
			if ts, err := strconv.ParseInt(parts[2], 10, 64); err == nil && ts > 0 {
				dt = time.Unix(ts, 0)
			}
		}
	}

	if dt.IsZero() {
		info, err := os.Stat(triggeredPath)
		if err != nil {
			return nil, err
		}
		dt = info.ModTime()
	}

	// Config overrides take precedence.
	if dw.SystemID.Valid {
		sysID = dw.SystemID.Int64
		systemLabel = ""
	}
	if dw.TalkgroupID.Valid {
		tgID = dw.TalkgroupID.Int64
	}
	if dw.Frequency.Valid {
		freq = dw.Frequency.Int64
	}

	return &ParsedCall{
		AudioFilePath:  triggeredPath,
		DateTime:       dt,
		SystemID:       sysID,
		SystemLabel:    systemLabel,
		TalkgroupID:    tgID,
		TalkgroupTitle: tgTitle,
		Frequency:      freq,
		Source:         source,
		Site:           site,
		Channel:        channel,
		Decoder:        decoder,
	}, nil
}

// parseSDRTrunkTags attempts to read ID3 tags from an MP3 file. Returns true
// if tags were successfully parsed, false otherwise.
func parseSDRTrunkTags(path string, systemLabel *string, tgID, freq, source *int64, dt *time.Time, tgTitle *string, site, channel, decoder *string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	if ext != ".mp3" {
		return false
	}

	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	m, err := readID3Tags(f)
	if err != nil || m == nil {
		return false
	}

	parsed := false

	// Artist: "<sourceID> [optional unit label]"
	if artist := m.artist; artist != "" {
		if s := reSDRArtist.FindStringSubmatch(artist); len(s) >= 2 {
			if i, err := strconv.ParseInt(s[1], 10, 64); err == nil && i > 0 {
				*source = i
				parsed = true
			}
		}
	}

	// Comment: "Date:YYYY-MM-DD HH:MM:SS.mmm;Frequency:NNNNN;System:label;"
	if comment := m.comment; comment != "" {
		if s := reSDRDate.FindStringSubmatch(comment); len(s) == 2 {
			if t, err := time.ParseInLocation("2006-01-02 15:04:05.999", s[1], time.Now().Location()); err == nil {
				*dt = t.UTC()
				parsed = true
			}
		}
		if s := reSDRFreq.FindStringSubmatch(comment); len(s) == 2 {
			if i, err := strconv.ParseInt(s[1], 10, 64); err == nil && i > 0 {
				*freq = i
				parsed = true
			}
		}
		if s := reSDRSystem.FindStringSubmatch(comment); len(s) == 2 {
			label := strings.TrimSpace(s[1])
			if label != "" {
				*systemLabel = label
				parsed = true
			}
		}
		if s := reSDRSite.FindStringSubmatch(comment); len(s) == 2 {
			if v := strings.TrimSpace(s[1]); v != "" {
				*site = v
				parsed = true
			}
		}
		if s := reSDRChannel.FindStringSubmatch(comment); len(s) == 2 {
			if v := strings.TrimSpace(s[1]); v != "" {
				*channel = v
				parsed = true
			}
		}
		if s := reSDRDecoder.FindStringSubmatch(comment); len(s) == 2 {
			if v := strings.TrimSpace(s[1]); v != "" {
				*decoder = v
				parsed = true
			}
		}
	}

	// Title: contains talkgroup ID (first numeric sequence) and full title text.
	if title := m.title; title != "" {
		if s := reSDRTgID.FindStringSubmatch(title); len(s) > 1 {
			if i, err := strconv.ParseInt(s[1], 10, 64); err == nil && i > 0 {
				*tgID = i
				parsed = true
			}
			// Strip the leading talkgroup ID and surrounding quotes from the title to get just the name.
			name := strings.TrimSpace(strings.TrimPrefix(title, s[1]))
			name = strings.Trim(name, "\"")
			name = strings.TrimSpace(name)
			if name != "" {
				*tgTitle = name
			}
		}
	}

	return parsed
}

// id3Meta holds the ID3 tag fields we care about.
type id3Meta struct {
	artist  string
	title   string
	comment string
}

// readID3Tags reads ID3v2 tags from an MP3 file. Returns nil if no tags found.
func readID3Tags(f *os.File) (*id3Meta, error) {
	// Read the first few KB to check for ID3 header.
	header := make([]byte, 10)
	if _, err := f.Read(header); err != nil {
		return nil, err
	}
	// Check for ID3v2 header: "ID3"
	if string(header[0:3]) != "ID3" {
		return nil, nil
	}

	// ID3v2 size is encoded as 4 syncsafe bytes.
	size := int(header[6])<<21 | int(header[7])<<14 | int(header[8])<<7 | int(header[9])
	if size <= 0 || size > 1<<20 {
		return nil, nil
	}

	// Re-read from start.
	if _, err := f.Seek(0, 0); err != nil {
		return nil, err
	}
	buf := make([]byte, 10+size)
	if _, err := io.ReadFull(f, buf); err != nil {
		return nil, err
	}

	version := header[3] // major version (3 = ID3v2.3, 4 = ID3v2.4)
	m := &id3Meta{}

	// Parse frames starting at offset 10.
	pos := 10
	for pos+10 <= len(buf) {
		frameID := string(buf[pos : pos+4])
		if frameID[0] == 0 {
			break // padding
		}

		var frameSize int
		if version == 4 {
			// ID3v2.4: syncsafe integer
			frameSize = int(buf[pos+4])<<21 | int(buf[pos+5])<<14 | int(buf[pos+6])<<7 | int(buf[pos+7])
		} else {
			// ID3v2.3: regular big-endian
			frameSize = int(buf[pos+4])<<24 | int(buf[pos+5])<<16 | int(buf[pos+6])<<8 | int(buf[pos+7])
		}

		if frameSize <= 0 || pos+10+frameSize > len(buf) {
			break
		}

		data := buf[pos+10 : pos+10+frameSize]
		text := extractID3Text(data)

		switch frameID {
		case "TPE1":
			m.artist = text
		case "TIT2":
			m.title = text
		case "COMM":
			m.comment = extractID3Comment(data)
		}

		pos += 10 + frameSize
	}

	if m.artist == "" && m.title == "" && m.comment == "" {
		return nil, nil
	}
	return m, nil
}

// extractID3Text extracts a text string from an ID3v2 text frame.
func extractID3Text(data []byte) string {
	if len(data) < 2 {
		return ""
	}
	encoding := data[0]
	payload := data[1:]
	switch encoding {
	case 0: // ISO-8859-1
		return strings.TrimRight(string(payload), "\x00")
	case 1: // UTF-16 with BOM
		return decodeUTF16(payload)
	case 3: // UTF-8
		return strings.TrimRight(string(payload), "\x00")
	default:
		return strings.TrimRight(string(payload), "\x00")
	}
}

// extractID3Comment extracts text from a COMM frame (encoding + 3-byte lang + short desc + \0 + text).
func extractID3Comment(data []byte) string {
	if len(data) < 5 {
		return ""
	}
	// Skip encoding byte + 3-byte language code.
	payload := data[4:]
	// Find the null terminator after the short description.
	idx := strings.IndexByte(string(payload), 0)
	if idx >= 0 && idx+1 < len(payload) {
		return strings.TrimRight(string(payload[idx+1:]), "\x00")
	}
	return strings.TrimRight(string(payload), "\x00")
}

// decodeUTF16 decodes a UTF-16 byte slice (with optional BOM) to a Go string.
func decodeUTF16(b []byte) string {
	if len(b) < 2 {
		return ""
	}
	// Detect BOM.
	bigEndian := true
	start := 0
	if b[0] == 0xFF && b[1] == 0xFE {
		bigEndian = false
		start = 2
	} else if b[0] == 0xFE && b[1] == 0xFF {
		bigEndian = true
		start = 2
	}
	var runes []rune
	for i := start; i+1 < len(b); i += 2 {
		var cp uint16
		if bigEndian {
			cp = uint16(b[i])<<8 | uint16(b[i+1])
		} else {
			cp = uint16(b[i+1])<<8 | uint16(b[i])
		}
		if cp == 0 {
			break
		}
		runes = append(runes, rune(cp))
	}
	return string(runes)
}

// ── RTL-SDR Airband ──────────────────────────────────────────────────────────

// reAirbandTimestamp matches RTLSDR-Airband filename timestamps:
//
//	Normal mode:   TEMPLATE_YYYYMMDD_HH.mp3
//	Split mode:    TEMPLATE_YYYYMMDD_HHMMSS.mp3
//	With freq:     TEMPLATE_YYYYMMDD_HHMMSS_FREQ.mp3
var reAirbandTimestamp = regexp.MustCompile(`_(\d{4})(\d{2})(\d{2})_(\d{2})(\d{2})?(\d{2})?(?:_(\d+))?(?:\.[^.]+)?$`)

func parseRTLSDRAirband(dw db.Dirmonitor, triggeredPath string) (*ParsedCall, error) {
	if !isAudioFile(triggeredPath) {
		return nil, nil
	}
	info, err := os.Stat(triggeredPath)
	if err != nil {
		return nil, err
	}

	var sysID, tgID, freq int64
	if dw.SystemID.Valid {
		sysID = dw.SystemID.Int64
	}
	if dw.TalkgroupID.Valid {
		tgID = dw.TalkgroupID.Int64
	}
	if dw.Frequency.Valid {
		freq = dw.Frequency.Int64
	}

	// Try to extract timestamp and frequency from the filename.
	dt := info.ModTime()
	base := filepath.Base(triggeredPath)
	if m := reAirbandTimestamp.FindStringSubmatch(base); m != nil {
		year, _ := strconv.Atoi(m[1])
		month, _ := strconv.Atoi(m[2])
		day, _ := strconv.Atoi(m[3])
		hour, _ := strconv.Atoi(m[4])
		var min, sec int
		if m[5] != "" {
			min, _ = strconv.Atoi(m[5])
		}
		if m[6] != "" {
			sec, _ = strconv.Atoi(m[6])
		}
		if year > 0 && month >= 1 && month <= 12 && day >= 1 && day <= 31 {
			dt = time.Date(year, time.Month(month), day, hour, min, sec, 0, time.UTC)
		}
		// Frequency from filename (include_freq mode) overrides config only when config is unset.
		if m[7] != "" && freq == 0 {
			if f, err := strconv.ParseInt(m[7], 10, 64); err == nil && f > 0 {
				freq = f
			}
		}
	}

	return &ParsedCall{
		AudioFilePath: triggeredPath,
		DateTime:      dt,
		SystemID:      sysID,
		TalkgroupID:   tgID,
		Frequency:     freq,
	}, nil
}

// ── DSDPlus ──────────────────────────────────────────────────────────────────

// Compiled regexes for DSD+ filename parsing.
var (
	reDSDDate   = regexp.MustCompile(`([0-9]+)$`)
	reDSDTime   = regexp.MustCompile(`^([0-9]+)`)
	reDSDSysNum = regexp.MustCompile(`^([0-9]+)-.+$`)
	reDSDNexSys = regexp.MustCompile(`^.([0-9]+)-[0-9]+$`)
	reDSDNexRAN = regexp.MustCompile(`RAN([0-9]+)`)
	reDSDP25Sys = regexp.MustCompile(`^[^.]+\.([^-]+)`)
)

// parseDSDPlus handles DSD+ recordings.
//
// Filename format uses underscore delimiters with bracket-escaped labels:
//
//	HHMMSS_[data]_MODE_CHANNEL_[TALKGROUP label]_[SOURCE label]
//
// Date is extracted from the parent folder name (YYYYMMDD suffix).
// MODE determines how the system ID is decoded from the CHANNEL segment.
func parseDSDPlus(dw db.Dirmonitor, triggeredPath string) (*ParsedCall, error) {
	if !isAudioFile(triggeredPath) {
		return nil, nil
	}

	base := strings.TrimSuffix(filepath.Base(triggeredPath), filepath.Ext(filepath.Base(triggeredPath)))
	dir := filepath.Dir(triggeredPath)

	// Split by underscores but treat [...] as atomic segments.
	meta := splitDSDPlusMeta(base)

	var dt time.Time

	// Extract date from parent folder (YYYYMMDD suffix) + time from filename (HHMMSS prefix).
	if d := reDSDDate.FindStringSubmatch(dir); len(d) == 2 && len(d[1]) == 8 {
		if t := reDSDTime.FindStringSubmatch(base); len(t) == 2 && len(t[1]) == 6 {
			dy, ye := strconv.Atoi(d[1][0:4])
			dm, me := strconv.Atoi(d[1][4:6])
			dd, de := strconv.Atoi(d[1][6:8])
			th, he := strconv.Atoi(t[1][0:2])
			tm, mie := strconv.Atoi(t[1][2:4])
			ts, se := strconv.Atoi(t[1][4:6])
			if ye == nil && me == nil && de == nil && he == nil && mie == nil && se == nil {
				dt = time.Date(dy, time.Month(dm), dd, th, tm, ts, 0, time.Now().Location()).UTC()
			}
		}
	}

	// Fall back to file ModTime if date extraction failed.
	if dt.IsZero() {
		info, err := os.Stat(triggeredPath)
		if err != nil {
			return nil, err
		}
		dt = info.ModTime()
	}

	var sysID, tgID, source int64
	var tgLabel string

	// Extract system ID from MODE + CHANNEL segments.
	if len(meta) > 3 {
		switch meta[2] {
		case "ConP(BS)", "DMR(BS)", "P25(BS)":
			if s := reDSDSysNum.FindStringSubmatch(meta[3]); len(s) > 1 {
				if i, err := strconv.ParseInt(s[1], 10, 64); err == nil {
					sysID = i
				}
			}
		case "NEXEDGE48(CB)", "NEXEDGE48(CS)", "NEXEDGE48(TB)",
			"NEXEDGE96(CB)", "NEXEDGE96(CS)", "NEXEDGE96(TB)":
			if s := reDSDNexSys.FindStringSubmatch(meta[3]); len(s) > 1 {
				if i, err := strconv.ParseInt(s[1], 10, 64); err == nil && i > 0 {
					sysID = i
				}
			} else if len(meta) > 4 {
				if s := reDSDNexRAN.FindStringSubmatch(meta[4]); len(s) > 1 {
					if i, err := strconv.ParseInt(s[1], 10, 64); err == nil && i > 0 {
						sysID = i
					}
				}
			}
		case "P25":
			if s := reDSDP25Sys.FindStringSubmatch(meta[3]); len(s) > 1 {
				if i, err := strconv.ParseInt(s[1], 16, 64); err == nil && i > 0 {
					sysID = i
				}
			}
		}
	}

	// Extract talkgroup ID and label from second-to-last segment.
	// Format: [TGID][optional label] — e.g. "[12345][Fire Dispatch]"
	if len(meta) >= 2 {
		brackets := extractBracketContents(meta[len(meta)-2])
		if len(brackets) > 0 {
			if i, err := strconv.ParseInt(brackets[0], 10, 64); err == nil && i > 0 {
				tgID = i
			}
		}
		if len(brackets) > 1 && hasAlphanumeric(brackets[1]) {
			tgLabel = brackets[1]
		}
	}

	// Extract source unit from last segment.
	if len(meta) >= 1 {
		brackets := extractBracketContents(meta[len(meta)-1])
		if len(brackets) > 0 {
			if i, err := strconv.ParseInt(brackets[0], 10, 64); err == nil && i > 0 {
				source = i
			}
		}
	}

	// Config overrides take precedence.
	if dw.SystemID.Valid {
		sysID = dw.SystemID.Int64
	}
	if dw.TalkgroupID.Valid {
		tgID = dw.TalkgroupID.Int64
	}

	return &ParsedCall{
		AudioFilePath:  triggeredPath,
		DateTime:       dt,
		SystemID:       sysID,
		TalkgroupID:    tgID,
		TalkgroupTitle: tgLabel,
		Source:         source,
	}, nil
}

// splitDSDPlusMeta splits a DSD+ filename by underscores, treating content
// within square brackets as atomic (not split). Brackets are preserved in
// the returned segments.
func splitDSDPlusMeta(base string) []string {
	meta := []string{""}
	inBracket := false
	ptr := 0
	for i := 0; i < len(base); i++ {
		ch := base[i]
		switch ch {
		case '[':
			inBracket = true
		case ']':
			inBracket = false
		}
		if !inBracket && ch == '_' {
			ptr++
			meta = append(meta, "")
		} else {
			meta[ptr] += string(ch)
		}
	}
	return meta
}

// ── ProScan ───────────────────────────────────────────────────────────────────

func parseProScan(dw db.Dirmonitor, triggeredPath string) (*ParsedCall, error) {
	if !isAudioFile(triggeredPath) {
		return nil, nil
	}
	info, err := os.Stat(triggeredPath)
	if err != nil {
		return nil, err
	}
	var sysID, tgID int64
	if dw.SystemID.Valid {
		sysID = dw.SystemID.Int64
	}
	if dw.TalkgroupID.Valid {
		tgID = dw.TalkgroupID.Int64
	}
	return &ParsedCall{
		AudioFilePath: triggeredPath,
		DateTime:      info.ModTime(),
		SystemID:      sysID,
		TalkgroupID:   tgID,
	}, nil
}

// ── Generic fallback ──────────────────────────────────────────────────────────

func parseGeneric(dw db.Dirmonitor, triggeredPath string) (*ParsedCall, error) {
	if !isAudioFile(triggeredPath) {
		return nil, nil
	}
	info, err := os.Stat(triggeredPath)
	if err != nil {
		return nil, err
	}
	var sysID, tgID int64
	if dw.SystemID.Valid {
		sysID = dw.SystemID.Int64
	}
	if dw.TalkgroupID.Valid {
		tgID = dw.TalkgroupID.Int64
	}
	return &ParsedCall{
		AudioFilePath: triggeredPath,
		DateTime:      info.ModTime(),
		SystemID:      sysID,
		TalkgroupID:   tgID,
	}, nil
}

// extractBracketContents collects the text inside each [...] pair in s.
// For "[54241][Fire Dispatch]" it returns ["54241", "Fire Dispatch"].
func extractBracketContents(s string) []string {
	var parts []string
	for {
		open := strings.IndexByte(s, '[')
		if open < 0 {
			break
		}
		close := strings.IndexByte(s[open+1:], ']')
		if close < 0 {
			break
		}
		inner := s[open+1 : open+1+close]
		if len(inner) > 0 {
			parts = append(parts, inner)
		}
		s = s[open+1+close+1:]
	}
	return parts
}

// hasAlphanumeric reports whether s contains at least one letter or digit.
// Used to filter out labels that are only punctuation/whitespace (e.g. "---").
func hasAlphanumeric(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return true
		}
	}
	return false
}
