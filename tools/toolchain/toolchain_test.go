package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestTaskfileVarsParsing(t *testing.T) {
	dir := t.TempDir()
	content := strings.Join([]string{
		"version: \"3\"",
		"",
		"vars:",
		"  TOOLCHAIN_DIR: tools/toolchain",
		"  DOTNET_VERSION: 8.0.425",
		"  GO_MIN_VERSION: \"1.27\"",
		"  LLVM_WINDOWS_URL: https://example.invalid/clang%2Bllvm.tar.xz",
		"",
		"env:",
		"  CC: '{{.TC_CC}}'",
		"",
		"tasks:",
		"  ignored:",
		"    cmds: [echo hi]",
		"",
	}, "\n")
	if err := os.WriteFile(filepath.Join(dir, taskfileMarker), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	vars, err := taskfileVars(dir)
	if err != nil {
		t.Fatal(err)
	}
	if vars["DOTNET_VERSION"] != "8.0.425" {
		t.Fatalf("DOTNET_VERSION = %q", vars["DOTNET_VERSION"])
	}
	if vars["GO_MIN_VERSION"] != "1.27" {
		t.Fatalf("引号未去掉：%q", vars["GO_MIN_VERSION"])
	}
	if vars["LLVM_WINDOWS_URL"] != "https://example.invalid/clang%2Bllvm.tar.xz" {
		t.Fatalf("URL 含冒号时应完整保留：%q", vars["LLVM_WINDOWS_URL"])
	}
	if _, ok := vars["CC"]; ok {
		t.Fatal("env: 块不应被当成 vars")
	}
	if _, ok := vars["ignored"]; ok {
		t.Fatal("tasks: 块不应被当成 vars")
	}
}

func TestDepValuePrefersEnvOverride(t *testing.T) {
	vars := map[string]string{"DOTNET_VERSION": "8.0.425"}
	var missing []string
	t.Setenv("TC_DOTNET_VERSION", "9.0.100")
	if got := depValue(vars, "DOTNET_VERSION", &missing); got != "9.0.100" {
		t.Fatalf("环境变量应覆盖 Taskfile：%q", got)
	}
	if got := depValue(vars, "GODOT_VERSION", &missing); got != "" {
		t.Fatalf("缺失声明应返回空：%q", got)
	}
	if len(missing) != 1 || missing[0] != "GODOT_VERSION" {
		t.Fatalf("缺失项记录不正确：%v", missing)
	}
}

func TestParseAndCompareVersions(t *testing.T) {
	cases := []struct {
		text string
		want string
	}{
		{"go version go1.27.0 windows/amd64", "1.27.0"},
		{"clang version 23.1.3 (https://github.com/llvm/llvm-project abc)", "23.1.3"},
		{"4.7.2.stable.mono.official.abc", "4.7.2"},
		{"8.0.425", "8.0.425"},
		{"no version here", ""},
	}
	for _, c := range cases {
		if got := parseVersion(c.text); got != c.want {
			t.Fatalf("parseVersion(%q) = %q, want %q", c.text, got, c.want)
		}
	}

	if compareVersions("1.27.0", "1.27") != 0 {
		t.Fatal("1.27.0 应等于 1.27")
	}
	if compareVersions("1.26.9", "1.27") != -1 {
		t.Fatal("1.26.9 应低于 1.27")
	}
	if compareVersions("23.1.3", "23.1.3") != 0 {
		t.Fatal("相同版本应相等")
	}
	if compareVersions("24.0.0", "23.1.3") != 1 {
		t.Fatal("24.0.0 应高于 23.1.3")
	}
}

func TestVersionLinePrefersToolLine(t *testing.T) {
	output := "Go: go1.27.0\nScanner: govulncheck@v1.1.4\nSee https://pkg.go.dev/golang.org/x/vuln/cmd/govulncheck for details.\n"
	line := versionLine("govulncheck", output)
	if !strings.Contains(line, "govulncheck") || parseVersion(line) != "1.1.4" {
		t.Fatalf("应取 govulncheck 所在行：%q", line)
	}
	if got := majorVersion("23.1.3"); got != "23" {
		t.Fatalf("majorVersion = %q", got)
	}
}

func TestFindGodotExeAndShim(t *testing.T) {
	root := t.TempDir()
	var target string
	switch runtime.GOOS {
	case "windows":
		target = filepath.Join(root, "Godot_v4.7.2-stable_mono_win64.exe")
	case "darwin":
		target = filepath.Join(root, "Godot_mono.app", "Contents", "MacOS", "Godot")
	default:
		target = filepath.Join(root, "Godot_v4.7.2-stable_mono_linux_x86_64", "Godot_v4.7.2-stable_mono_linux.x86_64")
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("fake"), 0o755); err != nil {
		t.Fatal(err)
	}
	found, err := findGodotExe(root)
	if err != nil {
		t.Fatalf("findGodotExe 失败：%v", err)
	}
	if filepath.Base(found) != filepath.Base(target) {
		t.Fatalf("找到的可执行文件不符：%s", found)
	}

	cfg := &Config{ToolsDir: filepath.Join(root, ".tools"), BinDir: filepath.Join(root, ".tools", "bin")}
	shim, err := writeShim(cfg, "godot", target)
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(shim)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), target) {
		t.Fatalf("shim 未指向真实二进制：%s", content)
	}
}

func TestFindMingwRoot(t *testing.T) {
	root := t.TempDir()
	lib := filepath.Join(root, "llvm-mingw-20260908-ucrt-x86_64", "lib", "clang", "23", "include")
	if err := os.MkdirAll(lib, 0o755); err != nil {
		t.Fatal(err)
	}
	found, err := findMingwRoot(root)
	if err != nil {
		t.Fatalf("findMingwRoot 失败：%v", err)
	}
	if filepath.Base(found) != "llvm-mingw-20260908-ucrt-x86_64" {
		t.Fatalf("MinGW 根目录不符：%s", found)
	}
}

func TestAssetURLs(t *testing.T) {
	got := dotnetURL("8.0.425", "win-x64", "zip")
	want := "https://builds.dotnet.microsoft.com/dotnet/Sdk/8.0.425/dotnet-sdk-8.0.425-win-x64.zip"
	if got != want {
		t.Fatalf("dotnetURL = %q, want %q", got, want)
	}
	got = godotURL("4.7.2", "win64")
	want = "https://github.com/godotengine/godot/releases/download/4.7.2-stable/Godot_v4.7.2-stable_mono_win64.zip"
	if got != want {
		t.Fatalf("godotURL = %q, want %q", got, want)
	}
}

func TestParseBootstrapArgs(t *testing.T) {
	opts, err := parseBootstrapArgs([]string{"--without", "godot,llvm", "--force"})
	if err != nil {
		t.Fatal(err)
	}
	if !opts.Force || !opts.Skip["godot"] || !opts.Skip["llvm"] {
		t.Fatalf("解析结果不符：%+v", opts)
	}
	if _, err := parseBootstrapArgs([]string{"--bogus"}); err == nil {
		t.Fatal("未知参数应报错")
	}
}

func TestRenderEnvFileIsBomlessAndQuotesOnlyWhenNeeded(t *testing.T) {
	content := renderEnvFile([]envEntry{
		{Key: "TC_PLAIN", Value: `C:\tools\bin`},
		{Key: "TC_SPACED", Value: `/opt/my tools/bin`},
		{Key: "TC_HASH", Value: `C:\a#b`},
		{Key: "TC_EMPTY", Value: ""},
	})
	if strings.HasPrefix(content, "\ufeff") {
		t.Fatal("env 文件不能带 BOM")
	}
	if strings.Contains(content, "\r") {
		t.Fatal("env 文件应使用 LF 换行")
	}
	if !strings.Contains(content, `TC_PLAIN=C:\tools\bin`) {
		t.Fatalf("普通路径不应被引号包裹：\n%s", content)
	}
	if !strings.Contains(content, "TC_SPACED=/opt/my tools/bin") {
		t.Fatalf("含空格路径不应加双引号：\n%s", content)
	}
	if !strings.Contains(content, "TC_HASH='C:\\a#b'") {
		t.Fatalf("含 # 的值应用单引号：\n%s", content)
	}
	if strings.Contains(content, "TC_EMPTY") {
		t.Fatalf("空值不应写入：\n%s", content)
	}
}

func TestSafeTargetRejectsEscape(t *testing.T) {
	dir := t.TempDir()
	if _, err := safeTarget(dir, "../escape.txt"); err == nil {
		t.Fatal("越界路径应报错")
	}
	if _, err := safeTarget(dir, filepath.Join("nested", "ok.txt")); err != nil {
		t.Fatalf("合法路径不应报错：%v", err)
	}
}

func TestVerifyFileDetectsMismatch(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "payload.bin")
	if err := os.WriteFile(path, []byte("riftcards"), 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte("riftcards"))
	good := hex.EncodeToString(sum[:])
	if err := verifyFile(path, "sha256", good); err != nil {
		t.Fatalf("正确哈希应通过：%v", err)
	}
	if err := verifyFile(path, "sha256", strings.Repeat("0", 64)); err == nil {
		t.Fatal("错误哈希应失败")
	}
}

func TestExtractZipAndTarGz(t *testing.T) {
	dir := t.TempDir()

	zipPath := filepath.Join(dir, "sample.zip")
	file, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(file)
	entry, err := zw.Create("nested/hello.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write([]byte("zip-ok")); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	zipOut := filepath.Join(dir, "zip-out")
	if err := extractArchive(zipPath, "zip", zipOut); err != nil {
		t.Fatalf("解压 zip 失败：%v", err)
	}
	content, err := os.ReadFile(filepath.Join(zipOut, "nested", "hello.txt"))
	if err != nil || string(content) != "zip-ok" {
		t.Fatalf("zip 内容不符：%q %v", content, err)
	}

	tarPath := filepath.Join(dir, "sample.tar.gz")
	out, err := os.Create(tarPath)
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(out)
	tw := tar.NewWriter(gz)
	payload := []byte("tar-ok")
	if err := tw.WriteHeader(&tar.Header{Name: "bin/tool", Mode: 0o755, Size: int64(len(payload))}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(payload); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	if err := out.Close(); err != nil {
		t.Fatal(err)
	}

	tarOut := filepath.Join(dir, "tar-out")
	if err := extractArchive(tarPath, "targz", tarOut); err != nil {
		t.Fatalf("解压 tar.gz 失败：%v", err)
	}
	info, err := os.Stat(filepath.Join(tarOut, "bin", "tool"))
	if err != nil {
		t.Fatal(err)
	}
	// Windows 不保留可执行位，只在类 Unix 平台断言。
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o111 == 0 {
		t.Fatalf("可执行位未保留：%v", info.Mode())
	}
}

func TestExtractRejectsTraversal(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "evil.zip")
	file, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(file)
	entry, err := zw.Create("../evil.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write([]byte("nope")); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if err := extractArchive(zipPath, "zip", filepath.Join(dir, "out")); err == nil {
		t.Fatal("越界条目应导致解压失败")
	}
}
