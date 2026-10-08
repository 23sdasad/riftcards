package pathutil

import (
	"errors"
	"testing"
)

func TestParseAndNormalize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		os    OS
		input string
		want  string
		abs   bool
	}{
		{name: "Windows drive cleanup", os: Windows, input: `C:\Users\alice\..\bob`, want: `C:\Users\bob`, abs: true},
		{name: "Windows accepts slashes", os: Windows, input: `C:/Users/alice`, want: `C:\Users\alice`, abs: true},
		{name: "Windows drive relative", os: Windows, input: `C:foo\..\bar`, want: `C:bar`},
		{name: "Windows rooted path", os: Windows, input: `\foo\bar`, want: `\foo\bar`, abs: true},
		{name: "Windows drive root", os: Windows, input: `C:\`, want: `C:\`, abs: true},
		{name: "Windows UNC cleanup", os: Windows, input: `\\server\share\dir\..`, want: `\\server\share`, abs: true},
		{name: "POSIX cleanup", os: Linux, input: `/var//log/../log`, want: `/var/log`, abs: true},
		{name: "POSIX backslash is name", os: Linux, input: `a\b`, want: `a\b`},
		{name: "POSIX parent preserved", os: Linux, input: `../a/./b`, want: `../a/b`},
		{name: "macOS POSIX cleanup", os: MacOS, input: `/Users/../Users/alice`, want: `/Users/alice`, abs: true},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			parsed, err := Parse(test.os, test.input)
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}
			if got := parsed.String(); got != test.want {
				t.Fatalf("String() = %q, want %q", got, test.want)
			}
			if got := parsed.IsAbs(); got != test.abs {
				t.Fatalf("IsAbs() = %t, want %t", got, test.abs)
			}
		})
	}
}

func TestParseRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		os    OS
		input string
		want  error
	}{
		{name: "unsupported OS", os: OS("plan9"), input: "x", want: ErrInvalidOS},
		{name: "NUL byte", os: Linux, input: "a\x00b", want: ErrInvalidPath},
		{name: "malformed UNC", os: Windows, input: `\\server`, want: ErrInvalidPath},
		{name: "device path unsupported", os: Windows, input: `\\?\C:\temp`, want: ErrInvalidPath},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := Parse(test.os, test.input)
			if !errors.Is(err, test.want) {
				t.Fatalf("Parse() error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestJoin(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		os         OS
		components []string
		want       string
	}{
		{name: "Linux absolute", os: Linux, components: []string{"/srv", "riftcards", "..", "data"}, want: "/srv/data"},
		{name: "macOS spaces", os: MacOS, components: []string{"/Users/me", "Library", "Application Support"}, want: "/Users/me/Library/Application Support"},
		{name: "Windows absolute", os: Windows, components: []string{`C:\base`, "dir", "..", "file.txt"}, want: `C:\base\file.txt`},
		{name: "Windows drive relative", os: Windows, components: []string{"C:", "dir", "file.txt"}, want: `C:dir\file.txt`},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, err := Join(test.os, test.components...)
			if err != nil {
				t.Fatalf("Join() error = %v", err)
			}
			if got != test.want {
				t.Fatalf("Join() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestJoinRejectsAbsoluteSuffix(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		os   OS
		base string
		next string
	}{
		{os: Linux, base: "/srv", next: "/etc"},
		{os: Windows, base: `C:\base`, next: `\outside`},
		{os: Windows, base: `C:\base`, next: `D:\outside`},
	} {
		if _, err := Join(test.os, test.base, test.next); !errors.Is(err, ErrAbsoluteComponent) {
			t.Fatalf("Join(%q, %q, %q) error = %v, want %v", test.os, test.base, test.next, err, ErrAbsoluteComponent)
		}
	}
}

func TestEqualUsesTargetCasePolicy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		os    OS
		left  string
		right string
		want  bool
	}{
		{name: "Windows case and slash", os: Windows, left: `C:\Base`, right: `c:/base`, want: true},
		{name: "Windows absolute differs from drive relative", os: Windows, left: `C:\Base`, right: `C:Base`, want: false},
		{name: "macOS case insensitive", os: MacOS, left: `/Users/Me`, right: `/users/me`, want: true},
		{name: "Linux case sensitive", os: Linux, left: `/Users/Me`, right: `/users/me`, want: false},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, err := Equal(test.os, test.left, test.right)
			if err != nil {
				t.Fatalf("Equal() error = %v", err)
			}
			if got != test.want {
				t.Fatalf("Equal() = %t, want %t", got, test.want)
			}
		})
	}
}

func TestWithin(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		os        OS
		base      string
		candidate string
		want      bool
	}{
		{name: "Windows child", os: Windows, base: `C:\base`, candidate: `c:\Base\child`, want: true},
		{name: "Windows sibling prefix", os: Windows, base: `C:\base`, candidate: `C:\base2`, want: false},
		{name: "Windows other drive", os: Windows, base: `C:\base`, candidate: `D:\base\child`, want: false},
		{name: "Windows drive relative is not confined", os: Windows, base: `C:\base`, candidate: `C:base\child`, want: false},
		{name: "Linux child", os: Linux, base: "/srv/app", candidate: "/srv/app/data", want: true},
		{name: "Linux sibling prefix", os: Linux, base: "/srv/app", candidate: "/srv/application", want: false},
		{name: "macOS case insensitive", os: MacOS, base: "/Users/me", candidate: "/users/ME/data", want: true},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, err := Within(test.os, test.base, test.candidate)
			if err != nil {
				t.Fatalf("Within() error = %v", err)
			}
			if got != test.want {
				t.Fatalf("Within() = %t, want %t", got, test.want)
			}
		})
	}
}

func TestWithinRequiresAbsoluteBase(t *testing.T) {
	t.Parallel()

	if _, err := Within(Linux, "relative", "relative/child"); !errors.Is(err, ErrBaseNotAbsolute) {
		t.Fatalf("Within() error = %v, want %v", err, ErrBaseNotAbsolute)
	}
}

func TestExecutableName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		os    OS
		input string
		want  string
	}{
		{name: "Windows adds exe", os: Windows, input: "riftcards-server", want: "riftcards-server.exe"},
		{name: "Windows preserves extension", os: Windows, input: "tool.cmd", want: "tool.cmd"},
		{name: "Linux unchanged", os: Linux, input: "riftcards-server", want: "riftcards-server"},
		{name: "macOS unchanged", os: MacOS, input: "riftcards-server", want: "riftcards-server"},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, err := ExecutableName(test.os, test.input)
			if err != nil {
				t.Fatalf("ExecutableName() error = %v", err)
			}
			if got != test.want {
				t.Fatalf("ExecutableName() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestExecutableNameRejectsPath(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		os    OS
		input string
	}{
		{os: Linux, input: "bin/tool"},
		{os: Windows, input: `bin\tool`},
		{os: Windows, input: ".."},
	} {
		if _, err := ExecutableName(test.os, test.input); !errors.Is(err, ErrInvalidPath) {
			t.Fatalf("ExecutableName(%q, %q) error = %v, want %v", test.os, test.input, err, ErrInvalidPath)
		}
	}
}

func TestUserHomeAndExpandUser(t *testing.T) {
	t.Parallel()

	windowsEnv := func(name string) string {
		switch name {
		case "USERPROFILE":
			return `C:\Users\Alice`
		default:
			return ""
		}
	}
	linuxEnv := func(name string) string {
		if name == "HOME" {
			return "/Users/alice"
		}
		return ""
	}

	if got, err := UserHome(Windows, windowsEnv); err != nil || got != `C:\Users\Alice` {
		t.Fatalf("UserHome(Windows) = %q, %v", got, err)
	}
	if got, err := ExpandUser(Windows, `~\data`, windowsEnv); err != nil || got != `C:\Users\Alice\data` {
		t.Fatalf("ExpandUser(Windows) = %q, %v", got, err)
	}
	if got, err := ExpandUser(Windows, `~/data`, windowsEnv); err != nil || got != `C:\Users\Alice\data` {
		t.Fatalf("ExpandUser(Windows slash) = %q, %v", got, err)
	}
	if got, err := UserHome(Linux, linuxEnv); err != nil || got != "/Users/alice" {
		t.Fatalf("UserHome(Linux) = %q, %v", got, err)
	}
	if got, err := ExpandUser(Linux, "~/data", linuxEnv); err != nil || got != "/Users/alice/data" {
		t.Fatalf("ExpandUser(Linux) = %q, %v", got, err)
	}
	if _, err := UserHome(Linux, func(string) string { return "" }); !errors.Is(err, ErrMissingHome) {
		t.Fatalf("UserHome() error = %v, want %v", err, ErrMissingHome)
	}
	if _, err := ExpandUser(Linux, "~other/data", linuxEnv); !errors.Is(err, ErrUnsupportedHome) {
		t.Fatalf("ExpandUser() error = %v, want %v", err, ErrUnsupportedHome)
	}
}

func TestUserHomeWindowsFallback(t *testing.T) {
	t.Parallel()

	env := func(name string) string {
		switch name {
		case "HOMEDRIVE":
			return "D:"
		case "HOMEPATH":
			return `\Profiles\Alice`
		default:
			return ""
		}
	}
	if got, err := UserHome(Windows, env); err != nil || got != `D:\Profiles\Alice` {
		t.Fatalf("UserHome(Windows fallback) = %q, %v", got, err)
	}
}

func TestSegmentsReturnsCopy(t *testing.T) {
	t.Parallel()

	parsed, err := Parse(Linux, "/a/b")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	segments := parsed.Segments()
	segments[0] = "changed"
	if got := parsed.String(); got != "/a/b" {
		t.Fatalf("String() after segment mutation = %q, want /a/b", got)
	}
}
