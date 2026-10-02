package main

import (
	"os"
	"sort"
	"strings"
	"syscall"
)

// updateEnv replaces or appends keys in a dotenv file and keeps every other line.
// The replacement keeps the previous owner and mode so a root domain service
// does not take the file away from the installation account.
func updateEnv(path string, values map[string]string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return os.ErrInvalid
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	pending := map[string]string{}
	for key, value := range values {
		pending[key] = value
	}
	var lines []string
	for _, line := range strings.Split(string(raw), "\n") {
		name, _, found := strings.Cut(line, "=")
		if found && !strings.HasPrefix(strings.TrimSpace(line), "#") {
			if value, exists := pending[name]; exists {
				lines = append(lines, name+"="+value)
				delete(pending, name)
				continue
			}
		}
		lines = append(lines, line)
	}
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	keys := make([]string, 0, len(pending))
	for key := range pending {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		lines = append(lines, key+"="+pending[key])
	}
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, []byte(strings.Join(lines, "\n")+"\n"), info.Mode().Perm()); err != nil {
		return err
	}
	if err := os.Chown(temporary, int(stat.Uid), int(stat.Gid)); err != nil {
		_ = os.Remove(temporary)
		return err
	}
	return os.Rename(temporary, path)
}

func envValue(path, key string) (string, bool) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	for _, line := range strings.Split(string(raw), "\n") {
		name, value, found := strings.Cut(line, "=")
		if found && name == key {
			return value, true
		}
	}
	return "", false
}
