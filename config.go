package main

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/mrcyjanek/cargadgetd_sdk/cfg"
	"gopkg.in/yaml.v3"
)

// configSearch, when non-nil, replaces the packaged/user load list (tests).
// configUser, when set, is the only file the GPS toggle patches.
var (
	configSearch []string
	configUser   string
)

type gpsFile struct {
	GPS *struct {
		Backend *string `yaml:"backend"`
	} `yaml:"gps"`
}

func configPaths() []string {
	if configSearch != nil {
		return configSearch
	}
	out := []string{
		"/usr/share/cargadgetd/config.yaml",
		"/etc/cargadgetd/config.yaml",
	}
	if p := userConfigPath(); p != "" {
		out = append(out, p)
	}
	return out
}

func userConfigPath() string {
	if configUser != "" {
		return configUser
	}
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil || home == "" {
			return ""
		}
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "cargadgetd", "config.yaml")
}

func loadLogOverlay() bool {
	v, ok := cfg.OwnBool("neon", "log_overlay")
	return ok && v
}

func loadGPS() string {
	kind := "geoclue"
	for _, p := range configPaths() {
		v, ok, err := readGPS(p)
		if err != nil || !ok || strings.TrimSpace(v) == "" {
			continue
		}
		kind = strings.TrimSpace(v)
	}
	return kind
}

func readGPS(path string) (string, bool, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", false, err
	}
	var f gpsFile
	if err := yaml.Unmarshal(b, &f); err != nil {
		return "", false, err
	}
	if f.GPS == nil || f.GPS.Backend == nil {
		return "", false, nil
	}
	return strings.TrimSpace(*f.GPS.Backend), true, nil
}

func patchGPS(path, kind string) error {
	return patchSectionKey(path, "gps", "backend", kind)
}

// patchSectionKey sets one scalar under a top-level section and leaves every
// other key and comment alone.
func patchSectionKey(path, section, key, val string) error {
	if path == "" {
		return os.ErrNotExist
	}
	b, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	head := section + ":"
	if err != nil || strings.TrimSpace(string(b)) == "" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		return os.WriteFile(path, []byte(head+"\n  "+key+": "+val+"\n"), 0o644)
	}
	lines := strings.Split(string(b), "\n")
	at := -1
	for i, ln := range lines {
		if indentWidth(ln) == 0 && strings.TrimSpace(ln) == head {
			at = i
			break
		}
	}
	if at < 0 {
		if len(lines) > 0 && lines[len(lines)-1] == "" {
			lines = lines[:len(lines)-1]
		}
		lines = append(lines, head, "  "+key+": "+val, "")
		return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644)
	}
	end := len(lines)
	found := -1
	for i := at + 1; i < len(lines); i++ {
		trim := strings.TrimSpace(lines[i])
		if trim == "" || strings.HasPrefix(trim, "#") {
			continue
		}
		if indentWidth(lines[i]) == 0 {
			end = i
			break
		}
		if strings.HasPrefix(trim, key+":") {
			found = i
		}
	}
	if found >= 0 {
		pad := leadingWS(lines[found])
		if pad == "" {
			pad = "  "
		}
		lines[found] = pad + key + ": " + val
	} else {
		line := "  " + key + ": " + val
		lines = append(lines[:end], append([]string{line}, lines[end:]...)...)
	}
	out := strings.Join(lines, "\n")
	if !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	return os.WriteFile(path, []byte(out), 0o644)
}

func indentWidth(s string) int {
	n := 0
	for _, r := range s {
		switch r {
		case ' ':
			n++
		case '\t':
			n += 2
		default:
			return n
		}
	}
	return n
}

func leadingWS(s string) string {
	i := 0
	for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
		i++
	}
	return s[:i]
}
