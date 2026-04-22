// Package config handles server startup configuration via CLI flags, environment variables, and optional JSON file.
//
// Configuration precedence: CLI flags > environment variables > JSON file > built-in defaults.
package config

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
)

// Version is set at build time via ldflags.
var Version = "1.0.0"

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

	flag.Usage = func() { PrintUsage(os.Stderr) }
	flag.Parse()

	// Capture which flags were explicitly set on the command line and their values,
	// before loadJSON/applyEnv can overwrite them.
	explicitFlags := make(map[string]string)
	flag.Visit(func(f *flag.Flag) {
		explicitFlags[f.Name] = f.Value.String()
	})

	// Load JSON file defaults (lowest precedence after built-in defaults).
	loadJSON(cfg)

	// Apply environment variables (higher precedence than JSON file).
	applyEnv(cfg)

	// Restore explicitly-set CLI flags (highest precedence).
	restoreExplicitFlags(cfg, explicitFlags)

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

// restoreExplicitFlags restores CLI flag values that were explicitly set by the user.
// This ensures CLI flags > env vars > JSON file > defaults.
func restoreExplicitFlags(cfg *Config, explicit map[string]string) {
	if v, ok := explicit["listen"]; ok {
		cfg.Listen = v
	}
	if v, ok := explicit["db-file"]; ok {
		cfg.DBFile = v
	}
	if v, ok := explicit["recordings-dir"]; ok {
		cfg.RecordingsDir = v
	}
	if v, ok := explicit["ssl-listen"]; ok {
		cfg.SSLListen = v
	}
	if v, ok := explicit["ssl-cert"]; ok {
		cfg.SSLCert = v
	}
	if v, ok := explicit["ssl-key"]; ok {
		cfg.SSLKey = v
	}
	if v, ok := explicit["ssl-auto-cert"]; ok {
		cfg.SSLAutoCert = v
	}
	if v, ok := explicit["admin-password"]; ok {
		cfg.AdminPassword = v
	}
	if v, ok := explicit["timezone"]; ok {
		cfg.Timezone = v
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

// PrintUsage writes the full CLI help text to w.
func PrintUsage(w io.Writer) {
	fmt.Fprintf(w, `OpenScanner — Radio Call Manager (v%s)

Usage:
  openscanner [flags]                   Start the server
  openscanner <command> [args]          Run a command
  openscanner help [command]            Show help for a command

Commands:
  setup               Install OpenScanner as a system service
  upgrade             Upgrade the installed binary (with service restart)
  config validate     Validate a JSON config file
  service doctor      Show service installation and status diagnostics
  login               Authenticate with a running server (saves token)
  logout              Remove saved authentication token
  change-password     Change the current user's password
  config-get [key]    Retrieve application settings (all or by key)
  config-set <k> <v>  Update an application setting
  user-add            Create a new user (interactive)
  user-remove <user>  Delete a user by username

Server Flags:
  --listen <addr>         HTTP listen address (default ":3022")
  --db-file <path>        SQLite database file path (default "openscanner.db")
  --recordings-dir <dir>  Directory for call audio recordings
  --timezone <tz>         IANA timezone (e.g. America/New_York)
  --config <path>         Path to JSON config file (default "openscanner.json")
  --config-save           Write current flags to JSON config file and exit
  --version               Print version and exit

SSL/TLS Flags:
  --ssl-listen <addr>     HTTPS listen address (default ":443" when SSL enabled)
  --ssl-cert <path>       TLS certificate file (PEM)
  --ssl-key <path>        TLS private key file (PEM)
  --ssl-auto-cert <host>  Enable Let's Encrypt for this domain

Service Flags:
  --service <action>      Service control: install, uninstall, start, stop, restart
  --admin-password <pw>   Reset first admin user's password on startup

CLI Flags (for remote commands):
  --server <url>          Server URL (default "http://localhost:3022")
                          Also: OPENSCANNER_SERVER env var

Environment Variables:
  OPENSCANNER_LISTEN          Equivalent to --listen
  OPENSCANNER_DB_FILE         Equivalent to --db-file
  OPENSCANNER_RECORDINGS_DIR  Equivalent to --recordings-dir
  OPENSCANNER_SSL_LISTEN      Equivalent to --ssl-listen
  OPENSCANNER_SSL_CERT        Equivalent to --ssl-cert
  OPENSCANNER_SSL_KEY         Equivalent to --ssl-key
  OPENSCANNER_SSL_AUTO_CERT   Equivalent to --ssl-auto-cert
  OPENSCANNER_ADMIN_PASSWORD  Equivalent to --admin-password
  OPENSCANNER_TIMEZONE        Equivalent to --timezone
  OPENSCANNER_SERVER          Server URL for CLI commands
  TZ                          Fallback timezone

Configuration Precedence:
  CLI flags > environment variables > JSON config file > built-in defaults

Run 'openscanner help <command>' for details on a specific command.
`, Version)
}

// commandHelp maps command names to their detailed help text.
var commandHelp = map[string]string{
	"setup": `Usage: openscanner setup [flags]

Install OpenScanner as a system service. Creates config file, database
directory, recordings directory, copies the binary, and registers the service.

Flags:
  --listen <addr>          HTTP listen address (default "127.0.0.1:3022")
  --db-file <path>         SQLite database file path
  --recordings-dir <dir>   Directory for call audio recordings
  --config <path>          Path to JSON config file
  --install-binary <path>  Path where executable is installed
  --interactive            Prompt for setup values interactively
  --force                  Overwrite/reinstall when setup already exists

Examples:
  openscanner setup --interactive
  openscanner setup --listen 0.0.0.0:8080 --force`,

	"upgrade": `Usage: openscanner upgrade [flags]

Upgrade the installed OpenScanner binary. Stops the service, copies the
new binary, and restarts if it was previously running.

Flags:
  --binary <path>          Path to new executable (default: current executable)
  --install-binary <path>  Installed executable path
  --config <path>          Path to JSON config file

Examples:
  openscanner upgrade
  openscanner upgrade --binary /tmp/openscanner-new`,

	"config": `Usage: openscanner config validate [flags]

Validate a JSON configuration file. Checks JSON syntax, required fields,
listen address format, and filesystem permissions.

Flags:
  --config <path>  Path to JSON config file

Examples:
  openscanner config validate
  openscanner config validate --config /etc/openscanner/openscanner.json`,

	"service": `Usage: openscanner service doctor

Show service installation status and diagnostics, including whether the
service is installed, running, and the default config/binary paths.

Examples:
  openscanner service doctor`,

	"login": `Usage: openscanner login [flags]

Authenticate with a running OpenScanner server. Prompts for username and
password interactively. On success, saves the JWT to ~/.openscanner-token.

Flags:
  --server <url>  Server URL (default "http://localhost:3022")

Examples:
  openscanner login
  openscanner login --server https://scanner.example.com`,

	"logout": `Usage: openscanner logout

Remove the saved authentication token (~/.openscanner-token).

Examples:
  openscanner logout`,

	"change-password": `Usage: openscanner change-password [flags]

Change the current user's password. Prompts for the current password
and the new password (with confirmation). Requires prior login.

Flags:
  --server <url>  Server URL (default "http://localhost:3022")

Examples:
  openscanner change-password`,

	"config-get": `Usage: openscanner config-get [key] [flags]

Retrieve application settings from the running server. Without a key,
prints all settings. With a key, prints only that setting. Requires
admin login.

Flags:
  --server <url>  Server URL (default "http://localhost:3022")

Examples:
  openscanner config-get
  openscanner config-get audioConversion`,

	"config-set": `Usage: openscanner config-set <key> <value> [flags]

Update an application setting on the running server. Requires admin login.

Flags:
  --server <url>  Server URL (default "http://localhost:3022")

Examples:
  openscanner config-set audioConversion 2
  openscanner config-set transcriptionEnabled true`,

	"user-add": `Usage: openscanner user-add [flags]

Create a new user on the running server. Prompts for username, password,
and role (admin or listener) interactively. Requires admin login.

Flags:
  --server <url>  Server URL (default "http://localhost:3022")

Examples:
  openscanner user-add`,

	"user-remove": `Usage: openscanner user-remove <username> [flags]

Delete a user by username on the running server. Requires admin login.

Flags:
  --server <url>  Server URL (default "http://localhost:3022")

Examples:
  openscanner user-remove jdoe`,
}

// RunHelp prints help for a specific command, or the general usage.
// Returns 0 on success, 1 if the command is unknown.
func RunHelp(topic string) int {
	if topic == "" {
		PrintUsage(os.Stdout)
		return 0
	}

	text, ok := commandHelp[topic]
	if !ok {
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\nRun 'openscanner help' for a list of commands.\n", topic)
		return 1
	}
	fmt.Println(text)
	return 0
}
