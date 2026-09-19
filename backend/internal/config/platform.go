package config

import (
	"os"
	"path/filepath"
	"runtime"
)

// Platform-specific default paths for setup tooling.
// These are computed once at init time based on runtime.GOOS.
var (
	DefaultConfigFile    string
	DefaultDBFile        string
	DefaultRecordingsDir string
	DefaultBinaryPath    string
)

func init() {
	switch runtime.GOOS {
	case "windows":
		programData := os.Getenv("ProgramData")
		if programData == "" {
			programData = `C:\ProgramData`
		}
		programFiles := os.Getenv("ProgramFiles")
		if programFiles == "" {
			programFiles = `C:\Program Files`
		}
		base := filepath.Join(programData, "Squelch")
		DefaultConfigFile = filepath.Join(base, "squelch.json")
		DefaultDBFile = filepath.Join(base, "squelch.db")
		DefaultRecordingsDir = filepath.Join(base, "recordings")
		DefaultBinaryPath = filepath.Join(programFiles, "Squelch", "squelch.exe")

	case "darwin":
		DefaultConfigFile = "/usr/local/etc/squelch/squelch.json"
		DefaultDBFile = "/usr/local/var/lib/squelch/squelch.db"
		DefaultRecordingsDir = "/usr/local/var/lib/squelch/recordings"
		DefaultBinaryPath = "/usr/local/bin/squelch"

	default: // linux, freebsd, etc.
		DefaultConfigFile = "/etc/squelch/squelch.json"
		DefaultDBFile = "/var/lib/squelch/squelch.db"
		DefaultRecordingsDir = "/var/lib/squelch/recordings"
		DefaultBinaryPath = "/usr/local/bin/squelch"
	}
}

// BinaryExtension returns ".exe" on Windows, empty string elsewhere.
func BinaryExtension() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}
