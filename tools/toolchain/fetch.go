package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// fetchArchive 返回缓存中已通过哈希校验的归档路径。
//
// 流程：命中缓存且哈希正确则直接复用；否则下载到 *.part，校验通过后再改名入缓存。
// 任何失败都不会在缓存目录留下未校验的文件。
func fetchArchive(c component, cacheDir string) (string, error) {
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return "", err
	}
	name := cacheName(c.URL)
	dest := filepath.Join(cacheDir, name)

	if _, err := os.Stat(dest); err == nil {
		if err := verifyFile(dest, c.HashAlgo, c.Hash); err == nil {
			fmt.Printf("  使用缓存 %s\n", relToRepo(dest))
			return dest, nil
		}
		fmt.Printf("  缓存哈希不符，重新下载 %s\n", name)
		if err := os.Remove(dest); err != nil {
			return "", err
		}
	}

	partial := dest + ".part"
	if err := download(c.URL, partial); err != nil {
		_ = os.Remove(partial)
		return "", err
	}
	if err := verifyFile(partial, c.HashAlgo, c.Hash); err != nil {
		_ = os.Remove(partial)
		return "", err
	}
	if err := os.Rename(partial, dest); err != nil {
		return "", err
	}
	return dest, nil
}

// cacheName 由 URL 得到缓存文件名：去掉查询串并还原百分号转义
// （例如 clang%2Bllvm 变为 clang+llvm）。
func cacheName(rawURL string) string {
	trimmed := strings.SplitN(rawURL, "?", 2)[0]
	base := filepath.Base(trimmed)
	decoded, err := url.PathUnescape(base)
	if err != nil {
		return base
	}
	return decoded
}

// downloadRounds 是每个通道的最大尝试轮数；轮与轮之间退避递增。
const downloadRounds = 3

// download 把 url 下载到 dest。依次尝试所有下载通道，整轮失败后退避重试。
func download(url, dest string) error {
	attempts := downloadAttempts()
	labels := make([]string, 0, len(attempts))
	for _, attempt := range attempts {
		labels = append(labels, attempt.label)
	}
	fmt.Printf("  下载通道：%s\n", strings.Join(labels, "、"))

	var lastErr error
	for round := 1; round <= downloadRounds; round++ {
		for _, attempt := range attempts {
			start := time.Now()
			err := downloadOnce(attempt.client, url, dest)
			if err == nil {
				fmt.Printf("  下载完成（%s，%s）\n", time.Since(start).Round(time.Second), attempt.label)
				return nil
			}
			lastErr = err
			_ = os.Remove(dest)
			fmt.Printf("  %s 失败：%v\n", attempt.label, err)
		}
		time.Sleep(time.Duration(round) * 2 * time.Second)
	}
	return fmt.Errorf("下载 %s 失败：%w", url, lastErr)
}

// downloadAttempt 是一条下载通道：一个标签（用于日志）加上一个 HTTP 客户端。
type downloadAttempt struct {
	label  string
	client *http.Client
}

// downloadAttempts 返回可用的下载通道，按优先级排序：
// 1. 默认网络（尊重 HTTP(S)_PROXY 等环境变量）；
// 2. Windows 系统代理（Go 不会自动读取，需要从注册表取）。
// 实测两条通道的可达性不同：有的主机直连能通，有的必须走代理，所以两条都试。
func downloadAttempts() []downloadAttempt {
	attempts := []downloadAttempt{{label: "默认网络", client: newHTTPClient("")}}
	if proxy := fallbackProxy(); proxy != "" {
		attempts = append(attempts, downloadAttempt{label: "系统代理 " + proxy, client: newHTTPClient(proxy)})
	}
	return attempts
}

// downloadOnce 用一条通道下载一次，只要非 200 或读写出错就返回错误。
func downloadOnce(client *http.Client, url, dest string) error {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "riftcards-toolchain")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %s", resp.Status)
	}

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()

	body := &progressReader{reader: resp.Body, total: resp.ContentLength}
	if _, err := io.Copy(out, body); err != nil {
		return err
	}
	body.finish()
	return out.Sync()
}

// newHTTPClient 基于默认传输创建客户端。proxy 为空时沿用 HTTP(S)_PROXY 等环境变量；
// 非空时显式使用该代理。
//
// 刻意保留默认的 HTTP/2：实测强制 HTTP/1.1 时 GitHub 资产会立刻 EOF，而默认传输能正常取数据。
func newHTTPClient(proxy string) *http.Client {
	transport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return &http.Client{Timeout: downloadTimeout}
	}
	cloned := transport.Clone()
	if proxy != "" {
		if parsed, err := url.Parse(proxy); err == nil {
			cloned.Proxy = http.ProxyURL(parsed)
		}
	}
	return &http.Client{Timeout: downloadTimeout, Transport: cloned}
}

// downloadTimeout 是单次下载的上限，覆盖最大的 LLVM MSI（约 640 MB）在慢网络下的情况。
const downloadTimeout = 30 * time.Minute

// proxyEnvVars 是 Go 默认会读取的代理环境变量。任一存在时，系统代理探测被跳过。
var proxyEnvVars = []string{"HTTPS_PROXY", "HTTP_PROXY", "ALL_PROXY", "https_proxy", "http_proxy"}

// fallbackProxy 在 Windows 上从注册表读取用户级代理设置，返回形如 http://host:port 的地址。
// 非 Windows、已设置代理环境变量、或未启用代理时返回空串。
func fallbackProxy() string {
	if runtime.GOOS != "windows" {
		return ""
	}
	for _, name := range proxyEnvVars {
		if os.Getenv(name) != "" {
			return ""
		}
	}
	if !strings.Contains(regQuery("ProxyEnable"), "0x1") {
		return ""
	}
	server := strings.TrimSpace(regQuery("ProxyServer"))
	if server == "" {
		return ""
	}
	server = pickProxyServer(server)
	if server == "" {
		return ""
	}
	if strings.Contains(server, "://") {
		return server
	}
	return "http://" + server
}

// pickProxyServer 从 ProxyServer 的值中选出一个地址。
//
// 注册表里有两种形式：直接是 host:port，或按协议分组，如 "http=h:p;https=h:p"。
// 分组形式优先取 https，其次取第一个可用项。
func pickProxyServer(value string) string {
	if !strings.Contains(value, "=") {
		return strings.TrimSpace(value)
	}
	fallback := ""
	for _, part := range strings.Split(value, ";") {
		scheme, host, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}
		host = strings.TrimSpace(host)
		if strings.EqualFold(strings.TrimSpace(scheme), "https") {
			return host
		}
		if fallback == "" {
			fallback = host
		}
	}
	return fallback
}

// regQuery 读取 HKCU 下 Internet Settings 的一个值，返回去掉类型名后的原始数据。
// 通过 reg.exe 读取，避免引入 golang.org/x/sys 依赖。
func regQuery(name string) string {
	out, err := runCapture("reg", "query", `HKCU\Software\Microsoft\Windows\CurrentVersion\Internet Settings`, "/v", name)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 3 && strings.EqualFold(fields[0], name) {
			return strings.Join(fields[2:], " ")
		}
	}
	return ""
}

type progressReader struct {
	reader io.Reader
	total  int64
	read   int64
	next   int64
}

func (p *progressReader) Read(buffer []byte) (int, error) {
	n, err := p.reader.Read(buffer)
	p.read += int64(n)
	if p.total > 0 && p.read >= p.next {
		fmt.Printf("\r  下载中 %3d%%（%.0f/%.0f MB）", p.read*100/p.total, float64(p.read)/1e6, float64(p.total)/1e6)
		p.next = p.read + p.total/10
	}
	return n, err
}

func (p *progressReader) finish() {
	if p.total > 0 {
		fmt.Printf("\r  下载中 100%%（%.0f/%.0f MB）\n", float64(p.total)/1e6, float64(p.total)/1e6)
	}
}

func verifyFile(path, algo, want string) error {
	got, err := hashFile(path, algo)
	if err != nil {
		return err
	}
	if !strings.EqualFold(got, want) {
		return fmt.Errorf("%s 校验失败：期望 %s，实际 %s", filepath.Base(path), want, got)
	}
	return nil
}

func hashFile(path, algo string) (string, error) {
	var h hash.Hash
	switch algo {
	case "sha256":
		h = sha256.New()
	case "sha512":
		h = sha512.New()
	default:
		return "", fmt.Errorf("未知哈希算法 %q", algo)
	}
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// extractArchive 按 kind 把 archivePath 展开到 destDir。
// kind 为 zip、targz（tar.gz）或 msi（仅 Windows）。归档内的路径必须留在 destDir 之内，
// 见 safeTarget。
func extractArchive(archivePath, kind, destDir string) error {
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return err
	}
	switch kind {
	case "zip":
		return extractZip(archivePath, destDir)
	case "targz":
		f, err := os.Open(archivePath)
		if err != nil {
			return err
		}
		defer func() { _ = f.Close() }()
		gz, err := gzip.NewReader(f)
		if err != nil {
			return err
		}
		defer func() { _ = gz.Close() }()
		return extractTar(gz, destDir)
	case "msi":
		return extractMSI(archivePath, destDir)
	default:
		return fmt.Errorf("未知归档类型 %q", kind)
	}
}

// extractMSI 用 Windows Installer 的管理安装（msiexec /a）把 MSI 展开到 destDir。
//
// 管理安装只复制文件，不注册产品、不写系统目录，也不需要管理员权限。
// 注意：msiexec 可能把工作交给 Windows Installer 服务异步完成，且对无效包也可能返回 0，
// 因此调用方必须以“产物是否出现”作为成功判据，而不能只看这里的返回值。
func extractMSI(archivePath, destDir string) error {
	if runtime.GOOS != "windows" {
		return fmt.Errorf("MSI 管理安装仅在 Windows 上可用")
	}
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return err
	}
	output, err := runCapture("msiexec", msiInstallArgs(archivePath, destDir)...)
	if err != nil {
		return fmt.Errorf("msiexec 管理安装失败（%v）：%s", err, strings.TrimSpace(output))
	}
	return nil
}

// msiInstallArgs 返回 msiexec 管理安装的参数：/a 管理安装、/qn 静默、/norestart 不重启、
// TARGETDIR 指定展开目录。独立成函数是为了便于单测。
func msiInstallArgs(archivePath, destDir string) []string {
	return []string{"/a", archivePath, "/qn", "/norestart", "TARGETDIR=" + destDir}
}

func extractZip(archivePath, destDir string) error {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer func() { _ = reader.Close() }()

	for _, file := range reader.File {
		target, err := safeTarget(destDir, file.Name)
		if err != nil {
			return err
		}
		mode := file.Mode()
		switch {
		case mode.IsDir():
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case mode&os.ModeSymlink != 0:
			link, err := readZipEntry(file)
			if err != nil {
				return err
			}
			if err := createSymlink(destDir, target, string(link)); err != nil {
				return err
			}
		default:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			src, err := file.Open()
			if err != nil {
				return err
			}
			err = writeFileFrom(target, src, mode.Perm())
			_ = src.Close()
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func readZipEntry(file *zip.File) ([]byte, error) {
	src, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer func() { _ = src.Close() }()
	return io.ReadAll(io.LimitReader(src, 4096))
}

func extractTar(reader io.Reader, destDir string) error {
	tr := tar.NewReader(reader)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		target, err := safeTarget(destDir, header.Name)
		if err != nil {
			return err
		}
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			if err := writeFileFrom(target, tr, os.FileMode(header.Mode).Perm()); err != nil {
				return err
			}
		case tar.TypeSymlink:
			if err := createSymlink(destDir, target, header.Linkname); err != nil {
				return err
			}
		default:
			// 设备、FIFO 等类型不需要，直接跳过。
		}
	}
}

func writeFileFrom(target string, src io.Reader, perm os.FileMode) error {
	if perm == 0 {
		perm = 0o644
	}
	out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, perm)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()
	if _, err := io.Copy(out, src); err != nil {
		return err
	}
	return nil
}

// createSymlink 只接受解析后仍位于 destDir 内的相对链接，避免归档越界。
func createSymlink(destDir, linkPath, linkTarget string) error {
	if filepath.IsAbs(linkTarget) {
		return fmt.Errorf("归档包含绝对符号链接 %s -> %s", linkPath, linkTarget)
	}
	if _, err := safeTarget(destDir, filepath.Join(filepath.Dir(linkPath), linkTarget)); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(linkPath), 0o755); err != nil {
		return err
	}
	_ = os.Remove(linkPath)
	return os.Symlink(linkTarget, linkPath)
}

// safeTarget 防止 zip/tar 中的 ../ 或绝对路径写到目标目录之外。
func safeTarget(destDir, name string) (string, error) {
	clean := filepath.Clean(filepath.FromSlash(name))
	if filepath.IsAbs(clean) {
		return "", fmt.Errorf("归档包含绝对路径 %s", name)
	}
	target := filepath.Join(destDir, clean)
	rel, err := filepath.Rel(destDir, target)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("归档路径越界 %s", name)
	}
	return target, nil
}

func relToRepo(path string) string {
	root, err := repoRoot()
	if err != nil {
		return path
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return path
	}
	return rel
}
