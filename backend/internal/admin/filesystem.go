package admin

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// FSDirectories lists directories under the given path for the dirmonitor
// picker UI. Path must be absolute; hidden (dotfile) and top-level system
// directories are filtered out.
func (o *Operations) FSDirectories(_ context.Context, params json.RawMessage, _ int64) (any, error) {
	var req struct {
		Path string `json:"path"`
	}
	if params != nil {
		_ = json.Unmarshal(params, &req)
	}
	if req.Path == "" {
		req.Path = "/"
	}

	clean := filepath.Clean(req.Path)
	if !filepath.IsAbs(clean) {
		return nil, UserError("path must be absolute")
	}

	info, err := os.Stat(clean)
	if err != nil {
		return nil, UserError("directory does not exist or is not accessible: " + err.Error())
	}
	if !info.IsDir() {
		return nil, UserError("path is not a directory: " + clean)
	}

	entries, err := os.ReadDir(clean)
	if err != nil {
		return nil, UserError("failed to read directory: " + err.Error())
	}

	type dirEntry struct {
		Name string `json:"name"`
		Path string `json:"path"`
	}

	dirs := make([]dirEntry, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if clean == "/" && hiddenTopLevelDirs[name] {
			continue
		}
		if strings.HasPrefix(name, ".") {
			continue
		}
		dirs = append(dirs, dirEntry{Name: name, Path: filepath.Join(clean, name)})
	}
	sort.Slice(dirs, func(i, j int) bool {
		return strings.ToLower(dirs[i].Name) < strings.ToLower(dirs[j].Name)
	})

	var parent *string
	if clean != "/" {
		p := filepath.Dir(clean)
		parent = &p
	}

	return map[string]any{
		"path":        clean,
		"parent":      parent,
		"directories": dirs,
	}, nil
}
