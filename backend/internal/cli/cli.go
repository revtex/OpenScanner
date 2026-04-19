// Package cli implements CLI subcommands that call the running OpenScanner HTTP API.
package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/term"
)

// tokenFileName is the file in the user's home directory that stores the JWT.
const tokenFileName = ".openscanner-token"

// Run checks os.Args for a CLI subcommand and executes it.
// Returns true if a subcommand was handled (caller should exit), false otherwise.
func Run() bool {
	args := nonFlagArgs()
	if len(args) == 0 {
		return false
	}

	serverURL := resolveServerURL()

	switch args[0] {
	case "login":
		os.Exit(runLogin(serverURL))
	case "logout":
		os.Exit(runLogout())
	case "change-password":
		os.Exit(runChangePassword(serverURL))
	case "config-get":
		key := ""
		if len(args) > 1 {
			key = args[1]
		}
		os.Exit(runConfigGet(serverURL, key))
	case "config-set":
		if len(args) < 3 {
			slog.Error("usage: openscanner config-set <key> <value>")
			os.Exit(1)
		}
		os.Exit(runConfigSet(serverURL, args[1], args[2]))
	case "user-add":
		os.Exit(runUserAdd(serverURL))
	case "user-remove":
		if len(args) < 2 {
			slog.Error("usage: openscanner user-remove <username>")
			os.Exit(1)
		}
		os.Exit(runUserRemove(serverURL, args[1]))
	default:
		return false
	}
	return true // unreachable due to os.Exit above, but keeps the compiler happy
}

// nonFlagArgs returns os.Args elements that are not flags (don't start with '-').
// It skips os.Args[0] (the binary name) and stops at the first non-flag argument.
func nonFlagArgs() []string {
	var result []string
	skipNext := false
	for _, arg := range os.Args[1:] {
		if skipNext {
			skipNext = false
			continue
		}
		if strings.HasPrefix(arg, "-") {
			// Flags like --listen=:3022 are self-contained.
			// Flags like --listen :3022 consume the next arg.
			if !strings.Contains(arg, "=") {
				skipNext = true
			}
			continue
		}
		result = append(result, arg)
	}
	return result
}

// resolveServerURL returns the server URL from --server flag, env var, or default.
func resolveServerURL() string {
	// Check for --server flag in os.Args.
	for i, arg := range os.Args[1:] {
		if arg == "--server" && i+2 < len(os.Args) {
			return strings.TrimRight(os.Args[i+2], "/")
		}
		if strings.HasPrefix(arg, "--server=") {
			return strings.TrimRight(strings.TrimPrefix(arg, "--server="), "/")
		}
	}
	if v := os.Getenv("OPENSCANNER_SERVER"); v != "" {
		return strings.TrimRight(v, "/")
	}
	return "http://localhost:3022"
}

// tokenPath returns the full path to the token file.
func tokenPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return tokenFileName
	}
	return filepath.Join(home, tokenFileName)
}

// loadToken reads the stored JWT from disk.
func loadToken() (string, error) {
	data, err := os.ReadFile(tokenPath())
	if err != nil {
		return "", fmt.Errorf("not logged in (run 'openscanner login' first)")
	}
	return strings.TrimSpace(string(data)), nil
}

// saveToken writes the JWT to disk with restricted permissions.
func saveToken(token string) error {
	return os.WriteFile(tokenPath(), []byte(token+"\n"), 0600)
}

// readPassword prompts for a password without echoing input.
func readPassword(prompt string) (string, error) {
	fmt.Print(prompt)
	pw, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println() // newline after hidden input
	if err != nil {
		return "", fmt.Errorf("failed to read password: %w", err)
	}
	return string(pw), nil
}

// readLine reads a line of text from stdin.
func readLine(prompt string) string {
	fmt.Print(prompt)
	var line string
	_, _ = fmt.Scanln(&line)
	return strings.TrimSpace(line)
}

// httpClient returns an HTTP client with sensible timeouts.
func httpClient() *http.Client {
	return &http.Client{
		Timeout: 30 * time.Second,
	}
}

// doRequest performs an HTTP request and returns the response body as a map.
func doRequest(method, url string, body interface{}, token string) (map[string]interface{}, int, error) {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to marshal request: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := httpClient().Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1 MiB limit
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("failed to read response: %w", err)
	}

	var result map[string]interface{}
	if len(respBody) > 0 {
		if err := json.Unmarshal(respBody, &result); err != nil {
			return nil, resp.StatusCode, fmt.Errorf("failed to parse response: %w", err)
		}
	}
	return result, resp.StatusCode, nil
}

// ---------- Subcommands ----------

func runLogin(serverURL string) int {
	username := readLine("Username: ")
	if username == "" {
		slog.Error("username is required")
		return 1
	}

	password, err := readPassword("Password: ")
	if err != nil {
		slog.Error("failed to read password", "error", err)
		return 1
	}

	body := map[string]string{
		"username": username,
		"password": password,
	}

	result, status, err := doRequest("POST", serverURL+"/api/auth/login", body, "")
	if err != nil {
		slog.Error("login failed", "error", err)
		return 1
	}
	if status != http.StatusOK {
		errMsg := "unknown error"
		if e, ok := result["error"].(string); ok {
			errMsg = e
		}
		slog.Error("login failed", "status", status, "error", errMsg)
		return 1
	}

	token, ok := result["token"].(string)
	if !ok || token == "" {
		slog.Error("login response missing token")
		return 1
	}

	if err := saveToken(token); err != nil {
		slog.Error("failed to save token", "error", err)
		return 1
	}

	user, _ := result["user"].(map[string]interface{})
	uname, _ := user["username"].(string)
	role, _ := user["role"].(string)
	fmt.Printf("Logged in as %s (%s)\n", uname, role)

	if needChange, ok := result["passwordNeedChange"].(bool); ok && needChange {
		fmt.Println("Note: password change required. Run 'openscanner change-password'.")
	}
	return 0
}

func runLogout() int {
	path := tokenPath()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		fmt.Println("Already logged out.")
		return 0
	}
	if err := os.Remove(path); err != nil {
		slog.Error("failed to remove token file", "error", err)
		return 1
	}
	fmt.Println("Logged out.")
	return 0
}

func runChangePassword(serverURL string) int {
	token, err := loadToken()
	if err != nil {
		slog.Error("authentication required", "error", err)
		return 1
	}

	currentPw, err := readPassword("Current password: ")
	if err != nil {
		slog.Error("failed to read password", "error", err)
		return 1
	}

	newPw, err := readPassword("New password: ")
	if err != nil {
		slog.Error("failed to read password", "error", err)
		return 1
	}

	confirmPw, err := readPassword("Confirm new password: ")
	if err != nil {
		slog.Error("failed to read password", "error", err)
		return 1
	}

	if newPw != confirmPw {
		slog.Error("passwords do not match")
		return 1
	}

	body := map[string]string{
		"currentPassword": currentPw,
		"newPassword":     newPw,
	}

	result, status, err := doRequest("PUT", serverURL+"/api/auth/password", body, token)
	if err != nil {
		slog.Error("change password failed", "error", err)
		return 1
	}
	if status != http.StatusOK {
		errMsg := "unknown error"
		if e, ok := result["error"].(string); ok {
			errMsg = e
		}
		slog.Error("change password failed", "status", status, "error", errMsg)
		return 1
	}

	fmt.Println("Password changed successfully.")
	return 0
}

func runConfigGet(serverURL, key string) int {
	token, err := loadToken()
	if err != nil {
		slog.Error("authentication required", "error", err)
		return 1
	}

	result, status, err := doRequest("GET", serverURL+"/api/admin/config", nil, token)
	if err != nil {
		slog.Error("failed to get config", "error", err)
		return 1
	}
	if status != http.StatusOK {
		errMsg := "unknown error"
		if e, ok := result["error"].(string); ok {
			errMsg = e
		}
		slog.Error("failed to get config", "status", status, "error", errMsg)
		return 1
	}

	if key != "" {
		val, ok := result[key]
		if !ok {
			slog.Error("key not found", "key", key)
			return 1
		}
		fmt.Printf("%s = %v\n", key, val)
		return 0
	}

	// Print all settings sorted by key.
	keys := make([]string, 0, len(result))
	for k := range result {
		keys = append(keys, k)
	}
	sortStrings(keys)
	for _, k := range keys {
		fmt.Printf("%s = %v\n", k, result[k])
	}
	return 0
}

func runConfigSet(serverURL, key, value string) int {
	token, err := loadToken()
	if err != nil {
		slog.Error("authentication required", "error", err)
		return 1
	}

	body := []map[string]string{
		{"key": key, "value": value},
	}

	result, status, err := doRequest("PUT", serverURL+"/api/admin/config", body, token)
	if err != nil {
		slog.Error("failed to set config", "error", err)
		return 1
	}
	if status != http.StatusOK {
		errMsg := "unknown error"
		if e, ok := result["error"].(string); ok {
			errMsg = e
		}
		slog.Error("failed to set config", "status", status, "error", errMsg)
		return 1
	}

	fmt.Printf("Set %s = %s\n", key, value)
	return 0
}

func runUserAdd(serverURL string) int {
	token, err := loadToken()
	if err != nil {
		slog.Error("authentication required", "error", err)
		return 1
	}

	username := readLine("Username: ")
	if username == "" {
		slog.Error("username is required")
		return 1
	}

	password, err := readPassword("Password: ")
	if err != nil {
		slog.Error("failed to read password", "error", err)
		return 1
	}

	confirmPw, err := readPassword("Confirm password: ")
	if err != nil {
		slog.Error("failed to read password", "error", err)
		return 1
	}

	if password != confirmPw {
		slog.Error("passwords do not match")
		return 1
	}

	role := readLine("Role (admin/listener) [listener]: ")
	if role == "" {
		role = "listener"
	}
	if role != "admin" && role != "listener" {
		slog.Error("role must be 'admin' or 'listener'")
		return 1
	}

	body := map[string]interface{}{
		"username": username,
		"password": password,
		"role":     role,
	}

	result, status, err := doRequest("POST", serverURL+"/api/admin/users", body, token)
	if err != nil {
		slog.Error("failed to create user", "error", err)
		return 1
	}
	if status != http.StatusCreated && status != http.StatusOK {
		errMsg := "unknown error"
		if e, ok := result["error"].(string); ok {
			errMsg = e
		}
		slog.Error("failed to create user", "status", status, "error", errMsg)
		return 1
	}

	fmt.Printf("User '%s' created with role '%s'.\n", username, role)
	return 0
}

func runUserRemove(serverURL, username string) int {
	token, err := loadToken()
	if err != nil {
		slog.Error("authentication required", "error", err)
		return 1
	}

	// First, list users to find the user ID by username.
	result, status, err := doRequest("GET", serverURL+"/api/admin/users", nil, token)
	if err != nil {
		slog.Error("failed to list users", "error", err)
		return 1
	}
	if status != http.StatusOK {
		errMsg := "unknown error"
		if e, ok := result["error"].(string); ok {
			errMsg = e
		}
		slog.Error("failed to list users", "status", status, "error", errMsg)
		return 1
	}

	// The users endpoint returns an array, but doRequest parses as map.
	// Re-request with raw handling.
	userID, err := findUserIDByName(serverURL, token, username)
	if err != nil {
		slog.Error("failed to find user", "username", username, "error", err)
		return 1
	}

	deleteURL := fmt.Sprintf("%s/api/admin/users/%d", serverURL, userID)
	delResult, delStatus, err := doRequest("DELETE", deleteURL, nil, token)
	if err != nil {
		slog.Error("failed to delete user", "error", err)
		return 1
	}
	if delStatus != http.StatusOK {
		errMsg := "unknown error"
		if e, ok := delResult["error"].(string); ok {
			errMsg = e
		}
		slog.Error("failed to delete user", "status", delStatus, "error", errMsg)
		return 1
	}

	fmt.Printf("User '%s' removed.\n", username)
	return 0
}

// findUserIDByName fetches the user list and returns the ID for the given username.
func findUserIDByName(serverURL, token, username string) (int64, error) {
	req, err := http.NewRequest("GET", serverURL+"/api/admin/users", nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := httpClient().Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return 0, err
	}

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("server returned %d", resp.StatusCode)
	}

	var users []struct {
		ID       int64  `json:"id"`
		Username string `json:"username"`
	}
	if err := json.Unmarshal(body, &users); err != nil {
		return 0, fmt.Errorf("failed to parse users list: %w", err)
	}

	for _, u := range users {
		if u.Username == username {
			return u.ID, nil
		}
	}
	return 0, fmt.Errorf("user '%s' not found", username)
}

// sortStrings sorts a string slice in place.
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
