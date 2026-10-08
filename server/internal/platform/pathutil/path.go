// Package pathutil provides deterministic path handling for supported target
// operating systems without relying on the host OS.
package pathutil

import (
	"errors"
	"fmt"
	"strings"
)

// OS identifies a path syntax and comparison policy.
type OS string

const (
	// Windows uses drive or UNC roots, backslashes, and case-insensitive names.
	Windows OS = "windows"
	// MacOS uses POSIX separators and case-insensitive names by default.
	MacOS OS = "macos"
	// Linux uses POSIX separators and case-sensitive names.
	Linux OS = "linux"
)

// EnvLookup resolves an environment variable.
type EnvLookup func(string) string

var (
	// ErrInvalidOS indicates an unsupported target operating system.
	ErrInvalidOS = errors.New("invalid path operating system")
	// ErrInvalidPath indicates malformed or unsupported path input.
	ErrInvalidPath = errors.New("invalid path")
	// ErrAbsoluteComponent indicates an absolute path was used as a join suffix.
	ErrAbsoluteComponent = errors.New("joined path component must be relative")
	// ErrBaseNotAbsolute indicates a confinement check used a relative base.
	ErrBaseNotAbsolute = errors.New("path base must be absolute")
	// ErrMissingHome indicates no valid user home could be resolved.
	ErrMissingHome = errors.New("user home is not available")
	// ErrUnsupportedHome indicates a home expansion form that is not supported.
	ErrUnsupportedHome = errors.New("unsupported user home expansion")
)

type rootKind uint8

const (
	rootNone rootKind = iota
	rootPOSIX
	rootWindows
	rootDrive
	rootUNC
)

// Path is a parsed, normalized path for a target OS.
type Path struct {
	os       OS
	kind     rootKind
	root     string
	absolute bool
	segments []string
}

// Parse validates and normalizes path text for the target OS.
func Parse(os OS, value string) (Path, error) {
	if !os.Valid() {
		return Path{}, fmt.Errorf("%w: %q", ErrInvalidOS, os)
	}
	if strings.IndexByte(value, 0) >= 0 {
		return Path{}, fmt.Errorf("%w: NUL byte", ErrInvalidPath)
	}

	if os == Windows {
		return parseWindows(value)
	}
	return parsePOSIX(os, value), nil
}

// Valid reports whether the OS has defined path semantics.
func (os OS) Valid() bool {
	switch os {
	case Windows, MacOS, Linux:
		return true
	default:
		return false
	}
}

// Separator returns the canonical path separator for the OS.
func (os OS) Separator() byte {
	if os == Windows {
		return '\\'
	}
	return '/'
}

// CaseSensitive reports the default file-name comparison policy.
func (os OS) CaseSensitive() bool {
	return os == Linux
}

// OS returns the path's target operating system.
func (p Path) OS() OS {
	return p.os
}

// IsAbs reports whether the path has an absolute root.
func (p Path) IsAbs() bool {
	return p.absolute
}

// Segments returns a copy of the normalized path segments.
func (p Path) Segments() []string {
	return append([]string(nil), p.segments...)
}

// String returns the canonical path text.
func (p Path) String() string {
	separator := string(p.os.Separator())
	joined := strings.Join(p.segments, separator)

	switch p.kind {
	case rootPOSIX:
		if joined == "" {
			return "/"
		}
		return "/" + joined
	case rootWindows:
		return "\\" + joined
	case rootDrive:
		if !p.absolute {
			return p.root + joined
		}
		if joined == "" {
			return p.root + "\\"
		}
		return p.root + "\\" + joined
	case rootUNC:
		if joined == "" {
			return p.root
		}
		return p.root + "\\" + joined
	default:
		return joined
	}
}

// Normalize parses and returns the canonical path text.
func Normalize(os OS, value string) (string, error) {
	parsed, err := Parse(os, value)
	if err != nil {
		return "", err
	}
	return parsed.String(), nil
}

// Join appends relative components to a base path and normalizes the result.
func Join(os OS, components ...string) (string, error) {
	if len(components) == 0 {
		return "", nil
	}

	base, err := Parse(os, components[0])
	if err != nil {
		return "", err
	}
	for _, component := range components[1:] {
		next, err := Parse(os, component)
		if err != nil {
			return "", err
		}
		if next.kind != rootNone || next.absolute {
			return "", fmt.Errorf("%w: %q", ErrAbsoluteComponent, component)
		}
		base.segments = cleanSegments(append(base.segments, next.segments...), base.absolute)
	}
	return base.String(), nil
}

// IsAbs reports whether value is absolute for the target OS.
func IsAbs(os OS, value string) (bool, error) {
	parsed, err := Parse(os, value)
	if err != nil {
		return false, err
	}
	return parsed.IsAbs(), nil
}

// Equal compares normalized paths using the target OS case policy.
func Equal(os OS, left, right string) (bool, error) {
	leftPath, err := Parse(os, left)
	if err != nil {
		return false, err
	}
	rightPath, err := Parse(os, right)
	if err != nil {
		return false, err
	}
	if leftPath.kind != rightPath.kind {
		return false, nil
	}
	if leftPath.absolute != rightPath.absolute {
		return false, nil
	}
	if !equalText(os, leftPath.root, rightPath.root) {
		return false, nil
	}
	return equalSegments(os, leftPath.segments, rightPath.segments), nil
}

// Within reports whether candidate is lexically confined to an absolute base.
func Within(os OS, base, candidate string) (bool, error) {
	basePath, err := Parse(os, base)
	if err != nil {
		return false, err
	}
	if !basePath.IsAbs() {
		return false, ErrBaseNotAbsolute
	}
	candidatePath, err := Parse(os, candidate)
	if err != nil {
		return false, err
	}
	if candidatePath.kind != basePath.kind {
		return false, nil
	}
	if !candidatePath.absolute {
		return false, nil
	}
	if !equalText(os, candidatePath.root, basePath.root) {
		return false, nil
	}
	if len(candidatePath.segments) < len(basePath.segments) {
		return false, nil
	}
	return equalSegments(os, basePath.segments, candidatePath.segments[:len(basePath.segments)]), nil
}

// ExecutableName returns a native executable file name for the target OS.
func ExecutableName(os OS, name string) (string, error) {
	parsed, err := Parse(os, name)
	if err != nil {
		return "", err
	}
	if parsed.kind != rootNone || len(parsed.segments) != 1 {
		return "", fmt.Errorf("%w: executable name %q must not contain a path", ErrInvalidPath, name)
	}
	base := parsed.segments[0]
	if base == "." || base == ".." {
		return "", fmt.Errorf("%w: executable name %q is reserved", ErrInvalidPath, name)
	}
	if os == Windows && extension(base) == "" {
		return base + ".exe", nil
	}
	return base, nil
}

// UserHome resolves and normalizes the user's home directory.
func UserHome(os OS, lookup EnvLookup) (string, error) {
	if !os.Valid() {
		return "", fmt.Errorf("%w: %q", ErrInvalidOS, os)
	}
	if lookup == nil {
		return "", ErrMissingHome
	}

	var home string
	if os == Windows {
		home = lookup("USERPROFILE")
		if home == "" {
			home = lookup("HOMEDRIVE") + lookup("HOMEPATH")
		}
	} else {
		home = lookup("HOME")
	}
	if home == "" {
		return "", ErrMissingHome
	}

	parsed, err := Parse(os, home)
	if err != nil {
		return "", err
	}
	if !parsed.IsAbs() {
		return "", fmt.Errorf("%w: home %q must be absolute", ErrMissingHome, home)
	}
	return parsed.String(), nil
}

// ExpandUser expands a leading home marker using the target OS environment.
func ExpandUser(os OS, value string, lookup EnvLookup) (string, error) {
	home, err := UserHome(os, lookup)
	if err != nil {
		return "", err
	}
	if value == "~" {
		return home, nil
	}

	prefixes := []string{"~/"}
	if os == Windows {
		prefixes = []string{"~\\", "~/"}
	}
	for _, prefix := range prefixes {
		if strings.HasPrefix(value, prefix) {
			return Join(os, home, value[len(prefix):])
		}
	}
	if strings.HasPrefix(value, "~") {
		return "", fmt.Errorf("%w: %q", ErrUnsupportedHome, value)
	}
	return Normalize(os, value)
}

func parsePOSIX(os OS, value string) Path {
	path := Path{os: os}
	if strings.HasPrefix(value, "/") {
		path.kind = rootPOSIX
		path.root = "/"
		path.absolute = true
	}
	path.segments = cleanSegments(strings.Split(value, "/"), path.absolute)
	return path
}

func parseWindows(value string) (Path, error) {
	normalized := strings.ReplaceAll(value, "/", "\\")
	path := Path{os: Windows}

	switch {
	case normalized == "":
		return path, nil
	case strings.HasPrefix(normalized, "\\\\"):
		if strings.HasPrefix(normalized, "\\\\?\\") {
			return Path{}, fmt.Errorf("%w: Windows device paths are unsupported", ErrInvalidPath)
		}
		parts := strings.Split(strings.TrimPrefix(normalized, "\\\\"), "\\")
		if len(parts) < 2 || !validUNCPart(parts[0]) || !validUNCPart(parts[1]) {
			return Path{}, fmt.Errorf("%w: malformed UNC path %q", ErrInvalidPath, value)
		}
		path.kind = rootUNC
		path.root = "\\\\" + parts[0] + "\\" + parts[1]
		path.absolute = true
		path.segments = cleanSegments(parts[2:], true)
	case hasDrivePrefix(normalized):
		path.kind = rootDrive
		path.root = strings.ToUpper(normalized[:1]) + ":"
		rest := normalized[2:]
		if strings.HasPrefix(rest, "\\") {
			path.absolute = true
			rest = strings.TrimLeft(rest, "\\")
		}
		path.segments = cleanSegments(strings.Split(rest, "\\"), path.absolute)
	case strings.HasPrefix(normalized, "\\"):
		path.kind = rootWindows
		path.root = "\\"
		path.absolute = true
		path.segments = cleanSegments(strings.Split(strings.TrimLeft(normalized, "\\"), "\\"), true)
	default:
		path.segments = cleanSegments(strings.Split(normalized, "\\"), false)
	}
	return path, nil
}

func cleanSegments(parts []string, absolute bool) []string {
	cleaned := make([]string, 0, len(parts))
	for _, part := range parts {
		switch part {
		case "", ".":
			continue
		case "..":
			if len(cleaned) > 0 && cleaned[len(cleaned)-1] != ".." {
				cleaned = cleaned[:len(cleaned)-1]
				continue
			}
			if !absolute {
				cleaned = append(cleaned, part)
			}
		default:
			cleaned = append(cleaned, part)
		}
	}
	return cleaned
}

func hasDrivePrefix(value string) bool {
	return len(value) >= 2 && isASCIIAlpha(value[0]) && value[1] == ':'
}

func isASCIIAlpha(value byte) bool {
	return value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z'
}

func validUNCPart(value string) bool {
	return value != "" && value != "." && value != ".."
}

func extension(name string) string {
	index := strings.LastIndexByte(name, '.')
	if index < 0 {
		return ""
	}
	return name[index:]
}

func equalText(os OS, left, right string) bool {
	if os.CaseSensitive() {
		return left == right
	}
	return strings.EqualFold(left, right)
}

func equalSegments(os OS, left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if !equalText(os, left[index], right[index]) {
			return false
		}
	}
	return true
}
