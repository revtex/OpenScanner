// Package dirwatch — per-recorder-type file parsers.
package dirwatch

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/openscanner/openscanner/internal/db"
)

// ParsedCall is the normalised metadata extracted by a recorder-specific parser.
type ParsedCall struct {
	AudioFilePath string    // absolute path to audio file on disk
	DateTime      time.Time // call timestamp
	SystemID      int64     // radio system ID (0 = unknown)
	TalkgroupID   int64     // radio talkgroup ID (0 = unknown)
	Frequency     int64     // Hz; 0 if unknown
	Duration      int64     // ms; 0 if unknown
	Source        int64     // source unit ID; 0 if unknown
	SourcesJSON   string    // JSON array string; "" if unknown
	FreqsJSON     string    // JSON array string; "" if unknown
	PatchesJSON   string    // JSON array string; "" if unknown
}

// ParseFunc parses a newly detected file for a given dirwatch config.
// It may return (nil, nil) when the file should be ignored (e.g. a sidecar
// whose matching audio has not yet arrived).
type ParseFunc func(dw db.Dirwatch, triggeredPath string) (*ParsedCall, error)

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
	case "sdrtrunk":
		return parseSDRTrunk
	case "rtlsdr-airband":
		return parseRTLSDRAirband
	case "dsdplus":
		return parseDSDPlus
	case "proscan":
		return parseProScan
	case "voxcall":
		return parseVoxCall
	default:
		return parseGeneric
	}
}

// ── Trunk Recorder ──────────────────────────────────────────────────────────

// trunkRecorderSidecar mirrors the JSON sidecar written by Trunk Recorder.
type trunkRecorderSidecar struct {
	StartTime         int64           `json:"start_time"`
	CallLength        float64         `json:"call_length"`
	Freq              int64           `json:"freq"`
	Talkgroup         int64           `json:"talkgroup"`
	SysNum            int64           `json:"sys_num"`
	Unit              int64           `json:"unit"`
	SrcList           json.RawMessage `json:"srcList"`
	FreqList          json.RawMessage `json:"freqList"`
	PatchedTalkgroups json.RawMessage `json:"patched_talkgroups"`
}

// parseTrunkRecorder handles both JSON sidecars and audio files.
//
//   - JSON triggered  →  parse sidecar, find matching audio; return nil if audio absent.
//   - Audio triggered →  look for .json sidecar; return nil if sidecar absent.
func parseTrunkRecorder(dw db.Dirwatch, triggeredPath string) (*ParsedCall, error) {
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

	return &ParsedCall{
		AudioFilePath: audioPath,
		DateTime:      time.Unix(sc.StartTime, 0),
		SystemID:      sc.SysNum,
		TalkgroupID:   sc.Talkgroup,
		Frequency:     sc.Freq,
		Duration:      int64(sc.CallLength * 1000),
		Source:        sc.Unit,
		SourcesJSON:   rawOrEmpty(sc.SrcList),
		FreqsJSON:     rawOrEmpty(sc.FreqList),
		PatchesJSON:   rawOrEmpty(sc.PatchedTalkgroups),
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

// parseSDRTrunk parses the filename pattern: <systemID>_<talkgroupID>_<unixTs>.<ext>
// If dirwatch overrides are set they take precedence.
func parseSDRTrunk(dw db.Dirwatch, triggeredPath string) (*ParsedCall, error) {
	if !isAudioFile(triggeredPath) {
		return nil, nil
	}

	name := filepath.Base(triggeredPath)
	stem := strings.TrimSuffix(name, filepath.Ext(name))
	parts := strings.SplitN(stem, "_", 3)

	var sysID, tgID, ts int64
	if len(parts) == 3 {
		sysID, _ = strconv.ParseInt(parts[0], 10, 64)
		tgID, _ = strconv.ParseInt(parts[1], 10, 64)
		ts, _ = strconv.ParseInt(parts[2], 10, 64)
	}

	if dw.SystemID.Valid {
		sysID = dw.SystemID.Int64
	}
	if dw.TalkgroupID.Valid {
		tgID = dw.TalkgroupID.Int64
	}

	var dt time.Time
	if ts > 0 {
		dt = time.Unix(ts, 0)
	} else {
		info, err := os.Stat(triggeredPath)
		if err != nil {
			return nil, err
		}
		dt = info.ModTime()
	}

	var freq int64
	if dw.Frequency.Valid {
		freq = dw.Frequency.Int64
	}

	return &ParsedCall{
		AudioFilePath: triggeredPath,
		DateTime:      dt,
		SystemID:      sysID,
		TalkgroupID:   tgID,
		Frequency:     freq,
	}, nil
}

// ── RTL-SDR Airband ──────────────────────────────────────────────────────────

func parseRTLSDRAirband(dw db.Dirwatch, triggeredPath string) (*ParsedCall, error) {
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
	return &ParsedCall{
		AudioFilePath: triggeredPath,
		DateTime:      info.ModTime(),
		SystemID:      sysID,
		TalkgroupID:   tgID,
		Frequency:     freq,
	}, nil
}

// ── DSDPlus ──────────────────────────────────────────────────────────────────

func parseDSDPlus(dw db.Dirwatch, triggeredPath string) (*ParsedCall, error) {
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

// ── ProScan ───────────────────────────────────────────────────────────────────

func parseProScan(dw db.Dirwatch, triggeredPath string) (*ParsedCall, error) {
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

// ── VoxCall ───────────────────────────────────────────────────────────────────

func parseVoxCall(dw db.Dirwatch, triggeredPath string) (*ParsedCall, error) {
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

func parseGeneric(dw db.Dirwatch, triggeredPath string) (*ParsedCall, error) {
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
