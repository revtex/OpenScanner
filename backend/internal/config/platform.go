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
		base := filepath.Join(programData, "OpenScanner")
		DefaultConfigFile = filepath.Join(base, "openscanner.json")
		DefaultDBFile = filepath.Join(base, "openscanner.db")
		DefaultRecordingsDir = filepath.Join(base, "recordings")
		DefaultBinaryPath = filepath.Join(programFiles, "OpenScanner", "openscanner.exe")

	case "darwin":
		DefaultConfigFile = "/usr/local/etc/openscanner/openscanner.json"
		DefaultDBFile = "/usr/local/var/lib/openscanner/openscanner.db"
		DefaultRecordingsDir = "/usr/local/var/lib/openscanner/recordings"
		DefaultBinaryPath = "/usr/local/bin/openscanner"

	default: // linux, freebsd, etc.
		DefaultConfigFile = "/etc/openscanner/openscanner.json"
		DefaultDBFile = "/var/lib/openscanner/openscanner.db"
		DefaultRecordingsDir = "/var/lib/openscanner/recordings"
		DefaultBinaryPath = "/usr/local/bin/openscanner"
	}
}

// BinaryExtension returns ".exe" on Windows, empty string elsewhere.
func BinaryExtension() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}
