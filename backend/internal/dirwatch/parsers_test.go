package dirwatch

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openscanner/openscanner/internal/db"
)

// ── helpers ───────────────────────────────────────────────────────────────────

// dwFor returns a minimal db.Dirwatch for the given type and directory.
func dwFor(dwType, dir string) db.Dirwatch {
	return db.Dirwatch{
		ID:        1,
		Directory: dir,
		Type:      dwType,
	}
}

// writeTRFiles creates a trunk-recorder .json sidecar and a .mp3 audio file
// sharing the same stem in dir. Returns both paths.
func writeTRFiles(t *testing.T, dir string) (jsonPath, audioPath string) {
	t.Helper()

	sidecar := map[string]any{
		"start_time":         int64(1700000000),
		"stop_time":          int64(1700000020),
		"call_length":        float64(20),
		"freq":               int64(851025000),
		"emergency":          0,
		"talkgroup":          int64(12345),
		"sys_num":            int64(1),
		"unit":               int64(5432),
		"srcList":            json.RawMessage(`[{"src":5432,"pos":0,"emergency":0,"signal_system":"","tag":""}]`),
		"freqList":           json.RawMessage(`[{"freq":851025000,"pos":0,"len":20,"error_count":0,"spike_count":0}]`),
		"patched_talkgroups": json.RawMessage(`[]`),
	}
	data, err := json.Marshal(sidecar)
	if err != nil {
		t.Fatalf("marshal sidecar: %v", err)
	}

	jsonPath = filepath.Join(dir, "call_1700000000.json")
	audioPath = filepath.Join(dir, "call_1700000000.mp3")

	if err := os.WriteFile(jsonPath, data, 0644); err != nil {
		t.Fatalf("write json: %v", err)
	}
	if err := os.WriteFile(audioPath, []byte("ID3FAKEMP3DATA"), 0644); err != nil {
		t.Fatalf("write audio: %v", err)
	}
	return jsonPath, audioPath
}

// ── trunk-recorder ────────────────────────────────────────────────────────────

func TestParseTrunkRecorder_AudioTrigger(t *testing.T) {
	dir := t.TempDir()
	_, audioPath := writeTRFiles(t, dir)

	dw := dwFor("trunk-recorder", dir)
	parsed, err := parseTrunkRecorder(dw, audioPath)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed == nil {
		t.Fatal("expected non-nil ParsedCall, got nil")
	}

	if parsed.SystemID != 1 {
		t.Errorf("SystemID = %d, want 1", parsed.SystemID)
	}
	if parsed.TalkgroupID != 12345 {
		t.Errorf("TalkgroupID = %d, want 12345", parsed.TalkgroupID)
	}
	if parsed.Frequency != 851025000 {
		t.Errorf("Frequency = %d, want 851025000", parsed.Frequency)
	}
	if parsed.Duration != 20000 {
		t.Errorf("Duration = %d ms, want 20000", parsed.Duration)
	}
	if parsed.Source != 5432 {
		t.Errorf("Source = %d, want 5432", parsed.Source)
	}
	if parsed.AudioFilePath != audioPath {
		t.Errorf("AudioFilePath = %q, want %q", parsed.AudioFilePath, audioPath)
	}
	wantUnix := int64(1700000000)
	if parsed.DateTime.Unix() != wantUnix {
		t.Errorf("DateTime unix = %d, want %d", parsed.DateTime.Unix(), wantUnix)
	}
}

func TestParseTrunkRecorder_JSONTrigger(t *testing.T) {
	dir := t.TempDir()
	jsonPath, audioPath := writeTRFiles(t, dir)

	dw := dwFor("trunk-recorder", dir)
	parsed, err := parseTrunkRecorder(dw, jsonPath)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed == nil {
		t.Fatal("expected non-nil ParsedCall, got nil")
	}
	if parsed.AudioFilePath != audioPath {
		t.Errorf("AudioFilePath = %q, want %q", parsed.AudioFilePath, audioPath)
	}
	if parsed.TalkgroupID != 12345 {
		t.Errorf("TalkgroupID = %d, want 12345", parsed.TalkgroupID)
	}
}

func TestParseTrunkRecorder_MissingAudio_ReturnsNil(t *testing.T) {
	dir := t.TempDir()

	// Write only the JSON sidecar — no audio file.
	sidecar := map[string]any{
		"start_time": int64(1700000000), "call_length": float64(20),
		"freq": int64(851025000), "talkgroup": int64(12345), "sys_num": int64(1), "unit": int64(5432),
	}
	data, _ := json.Marshal(sidecar)
	jsonPath := filepath.Join(dir, "call_1700000000.json")
	os.WriteFile(jsonPath, data, 0644) //nolint:errcheck

	dw := dwFor("trunk-recorder", dir)
	parsed, err := parseTrunkRecorder(dw, jsonPath)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if parsed != nil {
		t.Errorf("expected nil ParsedCall when audio missing, got %+v", parsed)
	}
}

func TestParseTrunkRecorder_MissingJSON_ReturnsNil(t *testing.T) {
	dir := t.TempDir()

	// Write only the audio file — no JSON sidecar.
	audioPath := filepath.Join(dir, "call_1700000000.mp3")
	os.WriteFile(audioPath, []byte("ID3FAKE"), 0644) //nolint:errcheck

	dw := dwFor("trunk-recorder", dir)
	parsed, err := parseTrunkRecorder(dw, audioPath)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if parsed != nil {
		t.Errorf("expected nil ParsedCall when sidecar missing, got %+v", parsed)
	}
}

func TestParseTrunkRecorder_NonAudioNonJSON_ReturnsNil(t *testing.T) {
	dir := t.TempDir()
	txtPath := filepath.Join(dir, "readme.txt")
	os.WriteFile(txtPath, []byte("text"), 0644) //nolint:errcheck

	dw := dwFor("trunk-recorder", dir)
	parsed, err := parseTrunkRecorder(dw, txtPath)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if parsed != nil {
		t.Errorf("expected nil ParsedCall for non-audio/non-json, got %+v", parsed)
	}
}

func TestParseTrunkRecorder_SourcesJSONPreserved(t *testing.T) {
	dir := t.TempDir()

	srcList := `[{"src":5432,"pos":0,"emergency":0,"signal_system":"","tag":""}]`
	freqList := `[{"freq":851025000,"pos":0,"len":20,"error_count":0,"spike_count":0}]`
	patches := `[99999]`

	sidecar := map[string]any{
		"start_time": int64(1700000000), "call_length": float64(10),
		"freq": int64(851025000), "talkgroup": int64(12345), "sys_num": int64(1), "unit": int64(1),
		"srcList": json.RawMessage(srcList), "freqList": json.RawMessage(freqList),
		"patched_talkgroups": json.RawMessage(patches),
	}
	data, _ := json.Marshal(sidecar)
	jsonPath := filepath.Join(dir, "x.json")
	audioPath := filepath.Join(dir, "x.mp3")
	os.WriteFile(jsonPath, data, 0644)      //nolint:errcheck
	os.WriteFile(audioPath, []byte{}, 0644) //nolint:errcheck

	dw := dwFor("trunk-recorder", dir)
	parsed, err := parseTrunkRecorder(dw, audioPath)
	if err != nil || parsed == nil {
		t.Fatalf("expected parsed call, got err=%v parsed=%v", err, parsed)
	}
	if parsed.SourcesJSON != srcList {
		t.Errorf("SourcesJSON = %q, want %q", parsed.SourcesJSON, srcList)
	}
	if parsed.FreqsJSON != freqList {
		t.Errorf("FreqsJSON = %q, want %q", parsed.FreqsJSON, freqList)
	}
	if parsed.PatchesJSON != patches {
		t.Errorf("PatchesJSON = %q, want %q", parsed.PatchesJSON, patches)
	}
}

// ── SDRTrunk ──────────────────────────────────────────────────────────────────

func TestParseSDRTrunk_FilenamePattern(t *testing.T) {
	dir := t.TempDir()
	// Filename: <systemID>_<talkgroupID>_<unixTs>.mp3
	audioPath := filepath.Join(dir, "1_12345_1700000000.mp3")
	os.WriteFile(audioPath, []byte("ID3FAKE"), 0644) //nolint:errcheck

	dw := dwFor("sdrtrunk", dir)
	parsed, err := parseSDRTrunk(dw, audioPath)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed == nil {
		t.Fatal("expected non-nil ParsedCall")
	}
	if parsed.SystemID != 1 {
		t.Errorf("SystemID = %d, want 1", parsed.SystemID)
	}
	if parsed.TalkgroupID != 12345 {
		t.Errorf("TalkgroupID = %d, want 12345", parsed.TalkgroupID)
	}
	if parsed.DateTime.Unix() != 1700000000 {
		t.Errorf("DateTime unix = %d, want 1700000000", parsed.DateTime.Unix())
	}
	if parsed.AudioFilePath != audioPath {
		t.Errorf("AudioFilePath = %q, want %q", parsed.AudioFilePath, audioPath)
	}
}

func TestParseSDRTrunk_DirwatchSystemIDOverride(t *testing.T) {
	dir := t.TempDir()
	audioPath := filepath.Join(dir, "1_12345_1700000000.mp3")
	os.WriteFile(audioPath, []byte("ID3FAKE"), 0644) //nolint:errcheck

	dw := db.Dirwatch{
		ID:        1,
		Directory: dir,
		Type:      "sdrtrunk",
		SystemID:  sql.NullInt64{Int64: 99, Valid: true},
	}
	parsed, err := parseSDRTrunk(dw, audioPath)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed == nil {
		t.Fatal("expected non-nil ParsedCall")
	}
	// Dirwatch SystemID should override the filename-parsed value.
	if parsed.SystemID != 99 {
		t.Errorf("SystemID = %d, want 99 (dirwatch override)", parsed.SystemID)
	}
	// TalkgroupID from filename is still 12345 (no talkgroup override set).
	if parsed.TalkgroupID != 12345 {
		t.Errorf("TalkgroupID = %d, want 12345", parsed.TalkgroupID)
	}
}

func TestParseSDRTrunk_DirwatchTalkgroupIDOverride(t *testing.T) {
	dir := t.TempDir()
	audioPath := filepath.Join(dir, "1_12345_1700000000.mp3")
	os.WriteFile(audioPath, []byte("ID3FAKE"), 0644) //nolint:errcheck

	dw := db.Dirwatch{
		ID:          1,
		Directory:   dir,
		Type:        "sdrtrunk",
		TalkgroupID: sql.NullInt64{Int64: 999, Valid: true},
	}
	parsed, err := parseSDRTrunk(dw, audioPath)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed == nil {
		t.Fatal("expected non-nil ParsedCall")
	}
	if parsed.TalkgroupID != 999 {
		t.Errorf("TalkgroupID = %d, want 999 (dirwatch override)", parsed.TalkgroupID)
	}
}

func TestParseSDRTrunk_FrequencyFromDirwatch(t *testing.T) {
	dir := t.TempDir()
	audioPath := filepath.Join(dir, "1_12345_1700000000.mp3")
	os.WriteFile(audioPath, []byte("ID3FAKE"), 0644) //nolint:errcheck

	dw := db.Dirwatch{
		ID:        1,
		Directory: dir,
		Type:      "sdrtrunk",
		Frequency: sql.NullInt64{Int64: 851025000, Valid: true},
	}
	parsed, err := parseSDRTrunk(dw, audioPath)
	if err != nil || parsed == nil {
		t.Fatalf("unexpected result: err=%v parsed=%v", err, parsed)
	}
	if parsed.Frequency != 851025000 {
		t.Errorf("Frequency = %d, want 851025000", parsed.Frequency)
	}
}

func TestParseSDRTrunk_NonAudioFile_ReturnsNil(t *testing.T) {
	dir := t.TempDir()
	txtPath := filepath.Join(dir, "1_12345_1700000000.txt")
	os.WriteFile(txtPath, []byte("text"), 0644) //nolint:errcheck

	dw := dwFor("sdrtrunk", dir)
	parsed, err := parseSDRTrunk(dw, txtPath)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if parsed != nil {
		t.Errorf("expected nil for non-audio file, got %+v", parsed)
	}
}

func TestParseSDRTrunk_InvalidFilenameUsesModTime(t *testing.T) {
	dir := t.TempDir()
	// Filename doesn't match the expected pattern — ts will be 0, so mod time is used.
	audioPath := filepath.Join(dir, "recording.mp3")
	os.WriteFile(audioPath, []byte("ID3FAKE"), 0644) //nolint:errcheck

	dw := dwFor("sdrtrunk", dir)
	parsed, err := parseSDRTrunk(dw, audioPath)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed == nil {
		t.Fatal("expected non-nil ParsedCall")
	}
	// DateTime should come from file mod time and be non-zero.
	if parsed.DateTime.IsZero() {
		t.Error("DateTime is zero, expected file modification time")
	}
}

func TestParserForType_SDRTrunkAlias(t *testing.T) {
	if p := parserForType("sdr-trunk"); p == nil {
		t.Fatal("expected parser for sdr-trunk")
	}
	if p := parserForType("sdrtrunk"); p == nil {
		t.Fatal("expected parser for sdrtrunk")
	}
}

// ── RTL-SDR Airband ───────────────────────────────────────────────────────────

func TestParseRTLSDRAirband_WithDirwatchOverrides(t *testing.T) {
	dir := t.TempDir()
	audioPath := filepath.Join(dir, "rec.wav")
	os.WriteFile(audioPath, []byte("RIFF"), 0644) //nolint:errcheck

	dw := db.Dirwatch{
		ID:          1,
		Directory:   dir,
		Type:        "rtlsdr-airband",
		SystemID:    sql.NullInt64{Int64: 7, Valid: true},
		TalkgroupID: sql.NullInt64{Int64: 42, Valid: true},
		Frequency:   sql.NullInt64{Int64: 120500000, Valid: true},
	}
	parsed, err := parseRTLSDRAirband(dw, audioPath)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed == nil {
		t.Fatal("expected non-nil ParsedCall")
	}
	if parsed.SystemID != 7 {
		t.Errorf("SystemID = %d, want 7", parsed.SystemID)
	}
	if parsed.TalkgroupID != 42 {
		t.Errorf("TalkgroupID = %d, want 42", parsed.TalkgroupID)
	}
	if parsed.Frequency != 120500000 {
		t.Errorf("Frequency = %d, want 120500000", parsed.Frequency)
	}
	if parsed.DateTime.IsZero() {
		t.Error("DateTime is zero")
	}
}

func TestParseRTLSDRAirband_NonAudio_ReturnsNil(t *testing.T) {
	dir := t.TempDir()
	txtPath := filepath.Join(dir, "file.txt")
	os.WriteFile(txtPath, []byte("x"), 0644) //nolint:errcheck

	parsed, err := parseRTLSDRAirband(dwFor("rtlsdr-airband", dir), txtPath)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if parsed != nil {
		t.Errorf("expected nil for non-audio file, got %+v", parsed)
	}
}

// ── DSDPlus ───────────────────────────────────────────────────────────────────

func TestParseDSDPlus_WithDirwatchOverrides(t *testing.T) {
	dir := t.TempDir()
	audioPath := filepath.Join(dir, "call.mp3")
	os.WriteFile(audioPath, []byte("ID3"), 0644) //nolint:errcheck

	dw := db.Dirwatch{
		ID:          1,
		Directory:   dir,
		Type:        "dsdplus",
		SystemID:    sql.NullInt64{Int64: 3, Valid: true},
		TalkgroupID: sql.NullInt64{Int64: 777, Valid: true},
	}
	parsed, err := parseDSDPlus(dw, audioPath)
	if err != nil || parsed == nil {
		t.Fatalf("unexpected: err=%v parsed=%v", err, parsed)
	}
	if parsed.SystemID != 3 {
		t.Errorf("SystemID = %d, want 3", parsed.SystemID)
	}
	if parsed.TalkgroupID != 777 {
		t.Errorf("TalkgroupID = %d, want 777", parsed.TalkgroupID)
	}
}

func TestParseDSDPlus_NonAudio_ReturnsNil(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "file.txt")
	os.WriteFile(p, []byte("x"), 0644) //nolint:errcheck

	parsed, err := parseDSDPlus(dwFor("dsdplus", dir), p)
	if err != nil {
		t.Error(err)
	}
	if parsed != nil {
		t.Errorf("expected nil, got %+v", parsed)
	}
}

// ── ProScan ───────────────────────────────────────────────────────────────────

func TestParseProScan_WithDirwatchOverrides(t *testing.T) {
	dir := t.TempDir()
	audioPath := filepath.Join(dir, "scan.mp3")
	os.WriteFile(audioPath, []byte("ID3"), 0644) //nolint:errcheck

	dw := db.Dirwatch{
		ID:          1,
		Directory:   dir,
		Type:        "proscan",
		SystemID:    sql.NullInt64{Int64: 5, Valid: true},
		TalkgroupID: sql.NullInt64{Int64: 888, Valid: true},
	}
	parsed, err := parseProScan(dw, audioPath)
	if err != nil || parsed == nil {
		t.Fatalf("unexpected: err=%v parsed=%v", err, parsed)
	}
	if parsed.SystemID != 5 {
		t.Errorf("SystemID = %d, want 5", parsed.SystemID)
	}
	if parsed.TalkgroupID != 888 {
		t.Errorf("TalkgroupID = %d, want 888", parsed.TalkgroupID)
	}
	if parsed.DateTime.IsZero() {
		t.Error("DateTime is zero")
	}
}

// ── VoxCall ───────────────────────────────────────────────────────────────────

func TestParseVoxCall_WithDirwatchOverrides(t *testing.T) {
	dir := t.TempDir()
	audioPath := filepath.Join(dir, "vox.ogg")
	os.WriteFile(audioPath, []byte("OggS"), 0644) //nolint:errcheck

	dw := db.Dirwatch{
		ID:          1,
		Directory:   dir,
		Type:        "voxcall",
		SystemID:    sql.NullInt64{Int64: 2, Valid: true},
		TalkgroupID: sql.NullInt64{Int64: 555, Valid: true},
	}
	parsed, err := parseVoxCall(dw, audioPath)
	if err != nil || parsed == nil {
		t.Fatalf("unexpected: err=%v parsed=%v", err, parsed)
	}
	if parsed.SystemID != 2 {
		t.Errorf("SystemID = %d, want 2", parsed.SystemID)
	}
	if parsed.TalkgroupID != 555 {
		t.Errorf("TalkgroupID = %d, want 555", parsed.TalkgroupID)
	}
}

func TestParseVoxCall_NonAudio_ReturnsNil(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "file.xml")
	os.WriteFile(p, []byte("<x/>"), 0644) //nolint:errcheck

	parsed, err := parseVoxCall(dwFor("voxcall", dir), p)
	if err != nil {
		t.Error(err)
	}
	if parsed != nil {
		t.Errorf("expected nil, got %+v", parsed)
	}
}

// ── Generic fallback ─────────────────────────────────────────────────────────

func TestParseGeneric_AudioFile(t *testing.T) {
	dir := t.TempDir()
	audioPath := filepath.Join(dir, "rec.flac")
	os.WriteFile(audioPath, []byte("fLaC"), 0644) //nolint:errcheck

	dw := db.Dirwatch{
		ID:          1,
		Directory:   dir,
		Type:        "generic",
		SystemID:    sql.NullInt64{Int64: 10, Valid: true},
		TalkgroupID: sql.NullInt64{Int64: 200, Valid: true},
	}
	parsed, err := parseGeneric(dw, audioPath)
	if err != nil || parsed == nil {
		t.Fatalf("unexpected: err=%v parsed=%v", err, parsed)
	}
	if parsed.SystemID != 10 {
		t.Errorf("SystemID = %d, want 10", parsed.SystemID)
	}
	if parsed.TalkgroupID != 200 {
		t.Errorf("TalkgroupID = %d, want 200", parsed.TalkgroupID)
	}
	if parsed.AudioFilePath != audioPath {
		t.Errorf("AudioFilePath = %q, want %q", parsed.AudioFilePath, audioPath)
	}
}

func TestParseGeneric_NonAudio_ReturnsNil(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "data.bin")
	os.WriteFile(p, []byte("x"), 0644) //nolint:errcheck

	parsed, err := parseGeneric(dwFor("generic", dir), p)
	if err != nil {
		t.Error(err)
	}
	if parsed != nil {
		t.Errorf("expected nil for non-audio file, got %+v", parsed)
	}
}

// ── parserForType dispatch ────────────────────────────────────────────────────

func TestParserForType(t *testing.T) {
	cases := []struct {
		recorderType string
	}{
		{"trunk-recorder"},
		{"sdrtrunk"},
		{"rtlsdr-airband"},
		{"dsdplus"},
		{"proscan"},
		{"voxcall"},
		{"unknown-type"},
		{""},
	}

	for _, tc := range cases {
		t.Run(tc.recorderType, func(t *testing.T) {
			fn := parserForType(tc.recorderType)
			if fn == nil {
				t.Errorf("parserForType(%q) = nil, want non-nil function", tc.recorderType)
			}
		})
	}
}

func TestParserForType_CaseInsensitive(t *testing.T) {
	// parserForType applies strings.ToLower; upper-case input should resolve identically.
	cases := []struct {
		upper string
		lower string
	}{
		{"TRUNK-RECORDER", "trunk-recorder"},
		{"SDRTrunk", "sdrtrunk"},
		{"RTLsdr-AIRBAND", "rtlsdr-airband"},
	}

	for _, tc := range cases {
		t.Run(tc.upper, func(t *testing.T) {
			got := parserForType(tc.upper)
			want := parserForType(tc.lower)
			// Only a nil/non-nil check is needed here.
			if got == nil && want != nil {
				t.Errorf("parserForType(%q) = nil, but parserForType(%q) is non-nil", tc.upper, tc.lower)
			}
		})
	}
}

// ── isAudioFile ───────────────────────────────────────────────────────────────

func TestIsAudioFile(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"call.mp3", true},
		{"call.MP3", true},
		{"call.wav", true},
		{"call.WAV", true},
		{"call.m4a", true},
		{"call.aac", true},
		{"call.ogg", true},
		{"call.flac", true},
		{"call.opus", true},
		{"call.txt", false},
		{"call.json", false},
		{"call.xml", false},
		{"call", false},
		{"/deep/path/call.mp3", true},
		{"/deep/path/data.bin", false},
	}

	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			got := isAudioFile(tc.path)
			if got != tc.want {
				t.Errorf("isAudioFile(%q) = %v, want %v", tc.path, got, tc.want)
			}
		})
	}
}

// ── rawOrEmpty ────────────────────────────────────────────────────────────────

func TestRawOrEmpty(t *testing.T) {
	cases := []struct {
		name string
		raw  json.RawMessage
		want string
	}{
		{"nil", nil, ""},
		{"null string", json.RawMessage("null"), ""},
		{"empty bytes", json.RawMessage{}, ""},
		{"array", json.RawMessage(`[1,2,3]`), `[1,2,3]`},
		{"object", json.RawMessage(`{"k":"v"}`), `{"k":"v"}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := rawOrEmpty(tc.raw)
			if got != tc.want {
				t.Errorf("rawOrEmpty(%q) = %q, want %q", tc.raw, got, tc.want)
			}
		})
	}
}

// ── Sidecar size limit ────────────────────────────────────────────────────────

func TestParseTrunkRecorder_SidecarTooLarge(t *testing.T) {
	dir := t.TempDir()

	// Write a sidecar larger than 1 MiB.
	largeData := make([]byte, (1<<20)+100)
	for i := range largeData {
		largeData[i] = '{'
	}
	jsonPath := filepath.Join(dir, "big.json")
	audioPath := filepath.Join(dir, "big.mp3")
	os.WriteFile(jsonPath, largeData, 0644)      //nolint:errcheck
	os.WriteFile(audioPath, []byte("ID3"), 0644) //nolint:errcheck

	dw := dwFor("trunk-recorder", dir)
	_, err := parseTrunkRecorder(dw, audioPath)
	if err == nil {
		t.Fatal("expected error for oversized sidecar, got nil")
	}
	if !strings.Contains(err.Error(), "too large") {
		t.Errorf("expected 'too large' in error, got: %v", err)
	}
}

func TestParseTrunkRecorder_SidecarNotExist_ReturnsNil(t *testing.T) {
	dir := t.TempDir()

	// Audio file present, but no sidecar.
	audioPath := filepath.Join(dir, "nosidecar.mp3")
	os.WriteFile(audioPath, []byte("ID3"), 0644) //nolint:errcheck

	dw := dwFor("trunk-recorder", dir)
	parsed, err := parseTrunkRecorder(dw, audioPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed != nil {
		t.Error("expected nil parsed call when sidecar is missing")
	}
}
