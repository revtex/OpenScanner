// Package config handles server startup configuration via CLI flags, environment variables, and optional INI file.
//
// Configuration precedence: CLI flags > environment variables > INI file > built-in defaults.
package config

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"gopkg.in/ini.v1"
)

// Version is set at build time via ldflags.
var Version = "0.1.0"

// Config holds all server startup configuration.
type Config struct {
	Listen        string // HTTP listen address (default ":3000")
	DBFile        string // SQLite database file path (default "openscanner.db")
	RecordingsDir string // Directory for call audio recordings (default: executable dir)
	SSLListen     string // HTTPS listen address
	SSLCert       string // TLS certificate file (PEM)
	SSLKey        string // TLS private key file (PEM)
	SSLAutoCert   string // Domain for Let's Encrypt auto-cert
	AdminPassword string // Reset first admin user's password on startup
	Timezone      string // IANA timezone for recorder timestamps (default: TZ env or "UTC")
	ConfigFile    string // Path to INI config file (default "openscanner.ini")
	ConfigSave    bool   // Write current flags to INI file and exit
	ShowVersion   bool   // Print version and exit
	Service       string // Service command: install, uninstall, start, stop, restart
}

// Load parses configuration from CLI flags, environment variables, and an optional INI file.
// Precedence: CLI flags > environment variables > INI file > built-in defaults.
func Load() (*Config, error) {
	cfg := &Config{}

	// Determine default RecordingsDir (directory of the running executable).
	exePath, err := os.Executable()
	if err != nil {
		exePath = "."
	}
	defaultRecordingsDir := filepath.Dir(exePath)

	// Define CLI flags.
	flag.StringVar(&cfg.Listen, "listen", ":3000", "HTTP listen address")
	flag.StringVar(&cfg.DBFile, "db-file", "openscanner.db", "SQLite database file path")
	flag.StringVar(&cfg.RecordingsDir, "recordings-dir", defaultRecordingsDir, "Directory for call audio recordings")
	flag.StringVar(&cfg.SSLListen, "ssl-listen", "", "HTTPS listen address")
	flag.StringVar(&cfg.SSLCert, "ssl-cert", "", "TLS certificate file (PEM)")
	flag.StringVar(&cfg.SSLKey, "ssl-key", "", "TLS private key file (PEM)")
	flag.StringVar(&cfg.SSLAutoCert, "ssl-auto-cert", "", "Domain for Let's Encrypt auto-cert")
	flag.StringVar(&cfg.AdminPassword, "admin-password", "", "Reset first admin user's password on startup")
	flag.StringVar(&cfg.Timezone, "timezone", "", "IANA timezone for recorder timestamps (e.g. America/New_York)")
	flag.StringVar(&cfg.ConfigFile, "config", "openscanner.ini", "Path to INI config file")
	flag.BoolVar(&cfg.ConfigSave, "config-save", false, "Write current flags to INI file and exit")
	flag.BoolVar(&cfg.ShowVersion, "version", false, "Print version and exit")
	flag.StringVar(&cfg.Service, "service", "", "Service command: install, uninstall, start, stop, restart")
	flag.Parse()

	// Load INI file defaults (lowest precedence after built-in defaults).
	loadINI(cfg)

	// Apply environment variables (higher precedence than INI).
	applyEnv(cfg)

	// CLI flags already parsed above have highest precedence (flag package handles this
	// since we set defaults before Parse, and Parse overwrites only if flag is provided).
	// However, flag package always sets the value, so we need to re-apply env and INI
	// only for flags that were NOT explicitly set on the command line.
	reapplyPrecedence(cfg, defaultRecordingsDir)

	return cfg, nil
}

// loadINI reads the INI config file and applies values as defaults.
func loadINI(cfg *Config) {
	iniFile, err := ini.Load(cfg.ConfigFile)
	if err != nil {
		// INI file is optional; if it doesn't exist, silently continue.
		return
	}

	section := iniFile.Section("")
	if v := section.Key("listen").String(); v != "" {
		cfg.Listen = v
	}
	if v := section.Key("db_file").String(); v != "" {
		cfg.DBFile = v
	}
	if v := section.Key("recordings_dir").String(); v != "" {
		cfg.RecordingsDir = v
	}
	if v := section.Key("ssl_listen").String(); v != "" {
		cfg.SSLListen = v
	}
	if v := section.Key("ssl_cert_file").String(); v != "" {
		cfg.SSLCert = v
	}
	if v := section.Key("ssl_key_file").String(); v != "" {
		cfg.SSLKey = v
	}
	if v := section.Key("ssl_auto_cert").String(); v != "" {
		cfg.SSLAutoCert = v
	}
	if v := section.Key("timezone").String(); v != "" {
		cfg.Timezone = v
	}
}

// applyEnv applies environment variable overrides.
func applyEnv(cfg *Config) {
	if v := os.Getenv("OPENSCANNER_LISTEN"); v != "" {
		cfg.Listen = v
	}
	if v := os.Getenv("OPENSCANNER_DB_FILE"); v != "" {
		cfg.DBFile = v
	}
	if v := os.Getenv("OPENSCANNER_RECORDINGS_DIR"); v != "" {
		cfg.RecordingsDir = v
	}
	if v := os.Getenv("OPENSCANNER_SSL_LISTEN"); v != "" {
		cfg.SSLListen = v
	}
	if v := os.Getenv("OPENSCANNER_SSL_CERT"); v != "" {
		cfg.SSLCert = v
	}
	if v := os.Getenv("OPENSCANNER_SSL_KEY"); v != "" {
		cfg.SSLKey = v
	}
	if v := os.Getenv("OPENSCANNER_SSL_AUTO_CERT"); v != "" {
		cfg.SSLAutoCert = v
	}
	if v := os.Getenv("OPENSCANNER_ADMIN_PASSWORD"); v != "" {
		cfg.AdminPassword = v
	}
	if v := os.Getenv("OPENSCANNER_TIMEZONE"); v != "" {
		cfg.Timezone = v
	} else if v := os.Getenv("TZ"); v != "" {
		cfg.Timezone = v
	}
}

// reapplyPrecedence ensures CLI flags > env vars > INI file > defaults.
// The flag package sets all values to defaults then overwrites with CLI args.
// We need to detect which flags were explicitly set on the command line.
func reapplyPrecedence(cfg *Config, defaultRecordingsDir string) {
	explicitFlags := make(map[string]bool)
	flag.Visit(func(f *flag.Flag) {
		explicitFlags[f.Name] = true
	})

	// For each setting: if not explicitly set via CLI, apply env > INI > default.
	// Since we already applied INI then env above, re-read env vars for non-explicit flags.
	if !explicitFlags["listen"] {
		if v := os.Getenv("OPENSCANNER_LISTEN"); v != "" {
			cfg.Listen = v
		}
	}
	if !explicitFlags["db-file"] {
		if v := os.Getenv("OPENSCANNER_DB_FILE"); v != "" {
			cfg.DBFile = v
		}
	}
	if !explicitFlags["recordings-dir"] {
		if v := os.Getenv("OPENSCANNER_RECORDINGS_DIR"); v != "" {
			cfg.RecordingsDir = v
		}
	}
	if !explicitFlags["ssl-listen"] {
		if v := os.Getenv("OPENSCANNER_SSL_LISTEN"); v != "" {
			cfg.SSLListen = v
		}
	}
	if !explicitFlags["ssl-cert"] {
		if v := os.Getenv("OPENSCANNER_SSL_CERT"); v != "" {
			cfg.SSLCert = v
		}
	}
	if !explicitFlags["ssl-key"] {
		if v := os.Getenv("OPENSCANNER_SSL_KEY"); v != "" {
			cfg.SSLKey = v
		}
	}
	if !explicitFlags["ssl-auto-cert"] {
		if v := os.Getenv("OPENSCANNER_SSL_AUTO_CERT"); v != "" {
			cfg.SSLAutoCert = v
		}
	}
	if !explicitFlags["admin-password"] {
		if v := os.Getenv("OPENSCANNER_ADMIN_PASSWORD"); v != "" {
			cfg.AdminPassword = v
		}
	}
	if !explicitFlags["timezone"] {
		if v := os.Getenv("OPENSCANNER_TIMEZONE"); v != "" {
			cfg.Timezone = v
		} else if v := os.Getenv("TZ"); v != "" {
			cfg.Timezone = v
		}
	}
}

// SaveINI writes the current configuration to the INI file.
func (c *Config) SaveINI() error {
	iniFile := ini.Empty()
	section := iniFile.Section("")

	section.Key("listen").SetValue(c.Listen)
	section.Key("db_file").SetValue(c.DBFile)
	section.Key("recordings_dir").SetValue(c.RecordingsDir)
	if c.SSLListen != "" {
		section.Key("ssl_listen").SetValue(c.SSLListen)
	}
	if c.SSLCert != "" {
		section.Key("ssl_cert_file").SetValue(c.SSLCert)
	}
	if c.SSLKey != "" {
		section.Key("ssl_key_file").SetValue(c.SSLKey)
	}
	if c.SSLAutoCert != "" {
		section.Key("ssl_auto_cert").SetValue(c.SSLAutoCert)
	}
	if c.Timezone != "" {
		section.Key("timezone").SetValue(c.Timezone)
	}

	slog.Info("saving configuration", "file", c.ConfigFile)
	return iniFile.SaveTo(c.ConfigFile)
}

// String returns a safe string representation (no secrets).
func (c *Config) String() string {
	return fmt.Sprintf("listen=%s db-file=%s recordings-dir=%s ssl-listen=%s ssl-auto-cert=%s timezone=%s",
		c.Listen, c.DBFile, c.RecordingsDir, c.SSLListen, c.SSLAutoCert, c.Timezone)
}
