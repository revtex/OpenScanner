// Package config handles server startup configuration via CLI flags, environment variables, and optional JSON file.
//
// Configuration precedence: CLI flags > environment variables > JSON file > built-in defaults.
package config

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
)

// Version is set at build time via ldflags.
var Version = "0.1.0"

// DefaultServiceConfigFile is the production default JSON config path used by setup tooling.
const DefaultServiceConfigFile = "/etc/openscanner/openscanner.json"

// Config holds all server startup configuration.
type Config struct {
	Listen        string // HTTP listen address (default ":3022")
	DBFile        string // SQLite database file path (default "openscanner.db")
	RecordingsDir string // Directory for call audio recordings (default: executable dir)
	SSLListen     string // HTTPS listen address
	SSLCert       string // TLS certificate file (PEM)
	SSLKey        string // TLS private key file (PEM)
	SSLAutoCert   string // Domain for Let's Encrypt auto-cert
	AdminPassword string // Reset first admin user's password on startup
	Timezone      string // IANA timezone for recorder timestamps (default: TZ env or "UTC")
	ConfigFile    string // Path to JSON config file (default "openscanner.json")
	ConfigSave    bool   // Write current flags to JSON config file and exit
	ShowVersion   bool   // Print version and exit
	Service       string // Service command: install, uninstall, start, stop, restart
}

type jsonFileConfig struct {
	Listen        string `json:"listen"`
	DBFile        string `json:"db_file"`
	RecordingsDir string `json:"recordings_dir"`
	SSLListen     string `json:"ssl_listen"`
	SSLCert       string `json:"ssl_cert_file"`
	SSLKey        string `json:"ssl_key_file"`
	SSLAutoCert   string `json:"ssl_auto_cert"`
	Timezone      string `json:"timezone"`
}

// Load parses configuration from CLI flags, environment variables, and an optional JSON file.
// Precedence: CLI flags > environment variables > JSON file > built-in defaults.
func Load() (*Config, error) {
	cfg := &Config{}

	// Determine default RecordingsDir (directory of the running executable).
	exePath, err := os.Executable()
	if err != nil {
		exePath = "."
	}
	defaultRecordingsDir := filepath.Dir(exePath)

	// Define CLI flags.
	flag.StringVar(&cfg.Listen, "listen", ":3022", "HTTP listen address")
	flag.StringVar(&cfg.DBFile, "db-file", "openscanner.db", "SQLite database file path")
	flag.StringVar(&cfg.RecordingsDir, "recordings-dir", defaultRecordingsDir, "Directory for call audio recordings")
	flag.StringVar(&cfg.SSLListen, "ssl-listen", "", "HTTPS listen address")
	flag.StringVar(&cfg.SSLCert, "ssl-cert", "", "TLS certificate file (PEM)")
	flag.StringVar(&cfg.SSLKey, "ssl-key", "", "TLS private key file (PEM)")
	flag.StringVar(&cfg.SSLAutoCert, "ssl-auto-cert", "", "Domain for Let's Encrypt auto-cert")
	flag.StringVar(&cfg.AdminPassword, "admin-password", "", "Reset first admin user's password on startup")
	flag.StringVar(&cfg.Timezone, "timezone", "", "IANA timezone for recorder timestamps (e.g. America/New_York)")
	flag.StringVar(&cfg.ConfigFile, "config", "openscanner.json", "Path to JSON config file")
	flag.BoolVar(&cfg.ConfigSave, "config-save", false, "Write current flags to JSON config file and exit")
	flag.BoolVar(&cfg.ShowVersion, "version", false, "Print version and exit")
	flag.StringVar(&cfg.Service, "service", "", "Service command: install, uninstall, start, stop, restart")
	flag.Parse()

	// Load JSON file defaults (lowest precedence after built-in defaults).
	loadJSON(cfg)

	// Apply environment variables (higher precedence than JSON file).
	applyEnv(cfg)

	// CLI flags already parsed above have highest precedence (flag package handles this
	// since we set defaults before Parse, and Parse overwrites only if flag is provided).
	// However, flag package always sets the value, so we need to re-apply env and JSON
	// only for flags that were NOT explicitly set on the command line.
	reapplyPrecedence(cfg)

	return cfg, nil
}

// loadJSON reads the JSON config file and applies values as defaults.
func loadJSON(cfg *Config) {
	data, err := os.ReadFile(cfg.ConfigFile)
	if err != nil {
		// Config file is optional; if it doesn't exist, silently continue.
		return
	}

	var fileCfg jsonFileConfig
	if err := json.Unmarshal(data, &fileCfg); err != nil {
		// Keep startup resilient if the config file is malformed.
		slog.Warn("failed to parse JSON config file", "file", cfg.ConfigFile, "error", err)
		return
	}

	if v := fileCfg.Listen; v != "" {
		cfg.Listen = v
	}
	if v := fileCfg.DBFile; v != "" {
		cfg.DBFile = v
	}
	if v := fileCfg.RecordingsDir; v != "" {
		cfg.RecordingsDir = v
	}
	if v := fileCfg.SSLListen; v != "" {
		cfg.SSLListen = v
	}
	if v := fileCfg.SSLCert; v != "" {
		cfg.SSLCert = v
	}
	if v := fileCfg.SSLKey; v != "" {
		cfg.SSLKey = v
	}
	if v := fileCfg.SSLAutoCert; v != "" {
		cfg.SSLAutoCert = v
	}
	if v := fileCfg.Timezone; v != "" {
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

// reapplyPrecedence ensures CLI flags > env vars > JSON file > defaults.
// The flag package sets all values to defaults then overwrites with CLI args.
// We need to detect which flags were explicitly set on the command line.
func reapplyPrecedence(cfg *Config) {
	explicitFlags := make(map[string]bool)
	flag.Visit(func(f *flag.Flag) {
		explicitFlags[f.Name] = true
	})

	// For each setting: if not explicitly set via CLI, apply env > JSON file > default.
	// Since we already applied JSON file then env above, re-read env vars for non-explicit flags.
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

// SaveJSON writes the current configuration to the JSON config file.
func (c *Config) SaveJSON() error {
	fileCfg := jsonFileConfig{
		Listen:        c.Listen,
		DBFile:        c.DBFile,
		RecordingsDir: c.RecordingsDir,
		SSLListen:     c.SSLListen,
		SSLCert:       c.SSLCert,
		SSLKey:        c.SSLKey,
		SSLAutoCert:   c.SSLAutoCert,
		Timezone:      c.Timezone,
	}

	data, err := json.MarshalIndent(fileCfg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	slog.Info("saving configuration", "file", c.ConfigFile)
	return os.WriteFile(c.ConfigFile, data, 0o644)
}

// String returns a safe string representation (no secrets).
func (c *Config) String() string {
	return fmt.Sprintf("listen=%s db-file=%s recordings-dir=%s ssl-listen=%s ssl-auto-cert=%s timezone=%s",
		c.Listen, c.DBFile, c.RecordingsDir, c.SSLListen, c.SSLAutoCert, c.Timezone)
}

// ValidateJSONFile validates a startup JSON config file and basic host filesystem assumptions.
func ValidateJSONFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("config file not found: %s", path)
		}
		return fmt.Errorf("read config file: %w", err)
	}

	var fileCfg jsonFileConfig
	if err := json.Unmarshal(data, &fileCfg); err != nil {
		return fmt.Errorf("parse JSON config: %w", err)
	}

	if fileCfg.Listen == "" {
		return fmt.Errorf("invalid config: listen is required")
	}
	if _, err := net.ResolveTCPAddr("tcp", fileCfg.Listen); err != nil {
		return fmt.Errorf("invalid listen address %q: %w", fileCfg.Listen, err)
	}

	if fileCfg.DBFile == "" {
		return fmt.Errorf("invalid config: db_file is required")
	}
	if err := ensureParentDirWritable(fileCfg.DBFile); err != nil {
		return fmt.Errorf("db_file path is not writable: %w", err)
	}

	if fileCfg.RecordingsDir == "" {
		return fmt.Errorf("invalid config: recordings_dir is required")
	}
	if err := ensureDirWritable(fileCfg.RecordingsDir); err != nil {
		return fmt.Errorf("recordings_dir is not writable: %w", err)
	}

	return nil
}

func ensureParentDirWritable(path string) error {
	dir := filepath.Dir(path)
	return ensureDirWritable(dir)
}

func ensureDirWritable(path string) error {
	if err := os.MkdirAll(path, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(path, ".openscanner-writecheck-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Remove(name)
}
