package agentcapabilities

import (
	"path"
	"strings"
	"unicode/utf8"
)

// ValidateSourceDirectories checks portable absolute source addresses without
// interpreting them on the Core host. Runtime authorizes and opens local paths.
func ValidateSourceDirectories(directories []string) error {
	if len(directories) > 50 {
		return ErrInvalid
	}
	seen := make(map[string]bool, len(directories))
	for _, directory := range directories {
		if len(directory) == 0 || len(directory) > 4096 || !utf8.ValidString(directory) || seen[directory] || !portableAbsoluteDirectory(directory) {
			return ErrInvalid
		}
		for _, b := range []byte(directory) {
			if b < 0x20 || b == 0x7f {
				return ErrInvalid
			}
		}
		seen[directory] = true
	}
	return nil
}

func portableAbsoluteDirectory(value string) bool {
	// Drive letters and UNC shares are wire syntax, not a Windows implementation.
	if len(value) >= 3 && ((value[0] >= 'A' && value[0] <= 'Z') || (value[0] >= 'a' && value[0] <= 'z')) && value[1] == ':' && (value[2] == '/' || value[2] == '\\') {
		rest := strings.ReplaceAll(value[2:], "\\", "/")
		return path.Clean(rest) == rest && !strings.ContainsAny(rest, ":*?\"<>|")
	}
	if strings.HasPrefix(value, "\\\\") || strings.HasPrefix(value, "//") {
		rest := strings.ReplaceAll(value[2:], "\\", "/")
		parts := strings.Split(rest, "/")
		if len(parts) < 2 || strings.ContainsAny(rest, ":*?\"<>|") {
			return false
		}
		for _, part := range parts {
			if part == "" || part == "." || part == ".." {
				return false
			}
		}
		return true
	}
	return strings.HasPrefix(value, "/") && !strings.Contains(value, "\\") && path.Clean(value) == value
}
