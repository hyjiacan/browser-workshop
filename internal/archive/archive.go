// Package archive provides unified archive extraction using pure Go.
// Supports .zip, .tar.gz, .tar.bz2, .tar.xz, .tar.zst, .7z formats.
package archive

import (
	"archive/tar"
	"archive/zip"
	"compress/bzip2"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/bodgit/sevenzip"
	"github.com/klauspost/compress/zstd"
	"github.com/ulikunitz/xz"
)

// maxExtractSize is the maximum total size of extracted files (10GB).
const maxExtractSize = 10 << 30

// safeFileMode masks out setuid/setgid/sticky bits from tar header modes.
func safeFileMode(mode int64) os.FileMode {
	return os.FileMode(mode) & 0o777
}

// supportedFormats lists archive format extensions we can handle (pure Go).
var supportedFormats = []string{
	".zip",
	".7z",
	".tar.gz",
	".tar.bz2",
	".tar.xz",
	".tar.zst",
	".tar",
	".gz",
	".bz2",
	".xz",
	".zst",
	".exe", // self-extracting archives (often zip-based)
	".dmg", // macOS disk images (zip-based)
	".apk", // Android APKs (zip-based)
	".jar", // Java archives (zip-based)
	".war", // Web archives (zip-based)
	".cab", // Windows cabinet files
}

// nestedArchiveFormats lists formats safe for recursive nested extraction.
var nestedArchiveFormats = []string{
	".zip",
	".7z",
	".tar.gz",
	".tar.bz2",
	".tar.xz",
	".tar.zst",
	".tar",
	".gz",
	".bz2",
	".xz",
	".zst",
	".jar",
	".war",
	".cab",
}

// SupportedFormats returns the list of supported archive format extensions.
func SupportedFormats() []string {
	result := make([]string, len(supportedFormats))
	copy(result, supportedFormats)
	return result
}

// IsSupportedFormat checks if a file has a supported archive extension.
func IsSupportedFormat(path string) bool {
	return hasSupportedFormat(path, supportedFormats)
}

// isNestedArchiveFormat checks if a file has a pure archive extension.
func isNestedArchiveFormat(path string) bool {
	return hasSupportedFormat(path, nestedArchiveFormats)
}

func hasSupportedFormat(path string, formats []string) bool {
	lower := strings.ToLower(path)
	for _, ext := range formats {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
}

// detectFormat returns the archive format extension from a file path.
func detectFormat(path string) string {
	lower := strings.ToLower(path)
	for _, ext := range supportedFormats {
		if strings.HasSuffix(lower, ext) {
			return ext
		}
	}
	ext := filepath.Ext(lower)
	if ext != "" {
		return ext
	}
	return ""
}

// Extract extracts an archive file to the destination directory using pure Go.
// No external tools (7z, etc.) are required.
func Extract(srcPath, destDir string) error {
	if _, err := os.Stat(srcPath); os.IsNotExist(err) {
		return fmt.Errorf("源文件不存在: %s", srcPath)
	}

	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("创建目标目录失败: %w", err)
	}

	format := detectFormat(srcPath)

	switch {
	case format == ".zip", format == ".jar", format == ".war", format == ".apk":
		return extractZip(srcPath, destDir)

	case format == ".7z":
		return extract7z(srcPath, destDir)

	case format == ".exe", format == ".dmg", format == ".cab":
		// Self-extracting archives are often zip-based
		if isZipFile(srcPath) {
			return extractZip(srcPath, destDir)
		}
		return fmt.Errorf("无法提取 %s 文件：不是有效的 zip 格式自解压包", format)

	case strings.HasPrefix(format, ".tar") || format == ".gz" || format == ".bz2" || format == ".xz" || format == ".zst":
		return extractTar(srcPath, destDir, format)

	default:
		return fmt.Errorf("不支持的压缩格式: %s", format)
	}
}

// ExtractRecursive extracts an archive and recursively extracts any nested archives.
func ExtractRecursive(srcPath, destDir string) (string, error) {
	if err := Extract(srcPath, destDir); err != nil {
		return "", fmt.Errorf("初始解压失败: %w", err)
	}
	if err := extractNested(destDir); err != nil {
		return "", fmt.Errorf("递归解压嵌套包失败: %w", err)
	}
	return destDir, nil
}

const maxExtractPasses = 10

func extractNested(rootDir string) error {
	for passes := 0; passes < maxExtractPasses; passes++ {
		found, err := extractNestedOnce(rootDir)
		if err != nil {
			return err
		}
		if !found {
			return nil
		}
	}
	return fmt.Errorf("超过最大嵌套解压次数 (%d)", maxExtractPasses)
}

func extractNestedOnce(rootDir string) (bool, error) {
	var archives []string

	err := filepath.WalkDir(rootDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasSuffix(path, ".bm_done") || strings.HasSuffix(path, ".extracted") {
			return nil
		}
		if isNestedArchiveFormat(path) {
			archives = append(archives, path)
		}
		return nil
	})
	if err != nil {
		return false, fmt.Errorf("扫描嵌套包失败: %w", err)
	}
	if len(archives) == 0 {
		return false, nil
	}

	extractedAny := false
	for _, archivePath := range archives {
		extractDir := deriveExtractDir(archivePath)
		if _, err := os.Stat(extractDir); err == nil {
			extractDir = extractDir + "_extracted"
		}
		if err := Extract(archivePath, extractDir); err != nil {
			continue
		}
		if err := os.Remove(archivePath); err != nil {
			_ = os.Rename(archivePath, archivePath+".extracted")
		}
		extractedAny = true
	}
	return extractedAny, nil
}

func deriveExtractDir(archivePath string) string {
	dir := filepath.Dir(archivePath)
	base := filepath.Base(archivePath)
	lowerBase := strings.ToLower(base)
	for _, ext := range supportedFormats {
		if strings.HasSuffix(lowerBase, ext) {
			name := base[:len(base)-len(ext)]
			if name == "" {
				name = base + "_contents"
			}
			return filepath.Join(dir, name)
		}
	}
	ext := filepath.Ext(base)
	name := base[:len(base)-len(ext)]
	if name == "" {
		name = base + "_contents"
	}
	return filepath.Join(dir, name)
}

// --- Zip extraction (native Go) ---

func extractZip(srcPath, destDir string) error {
	r, err := zip.OpenReader(srcPath)
	if err != nil {
		return fmt.Errorf("打开 zip 失败: %w", err)
	}
	defer r.Close()

	var totalSize int64
	for _, f := range r.File {
		fpath := filepath.Join(destDir, f.Name)
		cleanDest := filepath.Clean(destDir) + string(os.PathSeparator)
		if !strings.HasPrefix(filepath.Clean(fpath), cleanDest) {
			return fmt.Errorf("非法路径: %s", f.Name)
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(fpath, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(fpath), 0o755); err != nil {
			return err
		}
		dstFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}
		srcFile, err := f.Open()
		if err != nil {
			dstFile.Close()
			return err
		}
		n, err := io.Copy(dstFile, srcFile)
		srcFile.Close()
		dstFile.Close()
		if err != nil {
			return err
		}
		totalSize += n
		if totalSize > maxExtractSize {
			return fmt.Errorf("解压大小超过限制 (%d > %d)", totalSize, maxExtractSize)
		}
	}
	return nil
}

// --- 7z extraction (pure Go via sevenzip) ---

func extract7z(srcPath, destDir string) error {
	r, err := sevenzip.OpenReader(srcPath)
	if err != nil {
		return fmt.Errorf("打开 7z 文件失败: %w", err)
	}
	defer r.Close()

	var totalSize int64
	for _, f := range r.File {
		fpath := filepath.Join(destDir, f.Name)
		cleanDest := filepath.Clean(destDir) + string(os.PathSeparator)
		if !strings.HasPrefix(filepath.Clean(fpath), cleanDest) {
			return fmt.Errorf("非法路径: %s", f.Name)
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(fpath, 0o755); err != nil {
				return err
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(fpath), 0o755); err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			return fmt.Errorf("打开 7z 内文件失败: %w", err)
		}

		dstFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			rc.Close()
			return err
		}

		n, err := io.Copy(dstFile, rc)
		rc.Close()
		dstFile.Close()
		if err != nil {
			return err
		}
		totalSize += n
		if totalSize > maxExtractSize {
			return fmt.Errorf("解压大小超过限制 (%d > %d)", totalSize, maxExtractSize)
		}
	}
	return nil
}

// --- Tar extraction (pure Go) ---

func extractTar(srcPath, destDir string, format string) error {
	// Open the file
	f, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("打开文件失败: %w", err)
	}
	defer f.Close()

	// Determine the decompressor based on format
	var tr *tar.Reader
	switch {
	case format == ".tar.gz", format == ".tgz", format == ".gz":
		gr, err := newGzipReader(f)
		if err != nil {
			return fmt.Errorf("gzip 解压失败: %w", err)
		}
		defer gr.Close()
		tr = tar.NewReader(gr)

	case format == ".tar.bz2", format == ".bz2":
		br := bzip2.NewReader(f)
		tr = tar.NewReader(br)

	case format == ".tar.xz", format == ".xz":
		xr, err := newxzReader(f)
		if err != nil {
			return fmt.Errorf("xz 解压失败: %w", err)
		}
		defer xr.Close()
		tr = tar.NewReader(xr)

	case format == ".tar.zst", format == ".zst":
		zr, err := newZstdReader(f)
		if err != nil {
			return fmt.Errorf("zstd 解压失败: %w", err)
		}
		defer zr.Close()
		tr = tar.NewReader(zr)

	case format == ".tar":
		tr = tar.NewReader(f)

	default:
		return fmt.Errorf("不支持的 tar 格式: %s", format)
	}

	// Extract files
	var totalSize int64
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("读取 tar 失败: %w", err)
		}

		fpath := filepath.Join(destDir, header.Name)
		cleanDest := filepath.Clean(destDir) + string(os.PathSeparator)
		if !strings.HasPrefix(filepath.Clean(fpath), cleanDest) {
			return fmt.Errorf("非法路径: %s", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(fpath, safeFileMode(header.Mode)); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(fpath), 0o755); err != nil {
				return err
			}
			dstFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, safeFileMode(header.Mode))
			if err != nil {
				return err
			}
			n, err := io.Copy(dstFile, tr)
			dstFile.Close()
			if err != nil {
				return err
			}
			totalSize += n
			if totalSize > maxExtractSize {
				return fmt.Errorf("解压大小超过限制 (%d > %d)", totalSize, maxExtractSize)
			}
		}
	}
	return nil
}

// isZipFile checks if a file is actually a zip archive by reading its header.
func isZipFile(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	buf := make([]byte, 4)
	n, err := f.Read(buf)
	if err != nil || n < 4 {
		return false
	}
	return buf[0] == 'P' && buf[1] == 'K' && buf[2] == 0x03 && buf[3] == 0x04
}

// --- Helper wrappers for decompressors ---

// newGzipReader wraps os.File with gzip decompression.
func newGzipReader(f *os.File) (io.ReadCloser, error) {
	return gzip.NewReader(f)
}

// newxzReader wraps os.File with xz decompression.
func newxzReader(f *os.File) (io.ReadCloser, error) {
	r, err := xz.NewReader(f)
	if err != nil {
		return nil, err
	}
	return &xzReadCloser{Reader: r, file: f}, nil
}

type xzReadCloser struct {
	*xz.Reader
	file *os.File
}

func (x *xzReadCloser) Close() error { return nil }

// newZstdReader wraps os.File with zstd decompression.
func newZstdReader(f *os.File) (io.ReadCloser, error) {
	decoder, err := zstd.NewReader(f)
	if err != nil {
		return nil, err
	}
	return &zstdReadCloser{decompressor: decoder}, nil
}

type zstdReadCloser struct {
	decompressor *zstd.Decoder
}

func (z *zstdReadCloser) Read(p []byte) (int, error) {
	return z.decompressor.Read(p)
}

func (z *zstdReadCloser) Close() error {
	z.decompressor.Close()
	return nil
}

// --- Browser executable detection ---

// FindBrowserExe searches for a browser executable in the extracted directory.
func FindBrowserExe(rootDir, browserName, platform, arch string, executableNames []string) (string, error) {
	if len(executableNames) == 0 {
		exeName := browserName
		if platform == "windows" {
			exeName += ".exe"
		}
		executableNames = []string{exeName}
	}

	const maxDepth = 3
	var result string
	var walk func(dir string, depth int) bool
	walk = func(dir string, depth int) bool {
		if depth > maxDepth {
			return false
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			return false
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				for _, exeName := range executableNames {
					if strings.EqualFold(entry.Name(), exeName) {
						result = dir
						return true
					}
				}
			}
		}
		for _, entry := range entries {
			if entry.IsDir() {
				if walk(filepath.Join(dir, entry.Name()), depth+1) {
					return true
				}
			}
		}
		return false
	}
	if walk(rootDir, 0) {
		return result, nil
	}
	return "", fmt.Errorf("未找到浏览器可执行文件 (搜索 %d 层: %s)", maxDepth, strings.Join(executableNames, ", "))
}

// FindContentDir finds the actual content directory within an extracted archive.
func FindContentDir(root, browserName, platform, arch string, exeCandidates []string) (string, error) {
	if len(exeCandidates) > 0 {
		if dir, err := FindBrowserExe(root, browserName, platform, arch, exeCandidates); err == nil {
			return dir, nil
		}
	}
	return findContentDirHeuristic(root)
}

func findContentDirHeuristic(root string) (string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return "", err
	}
	var dirs []os.DirEntry
	var files []os.DirEntry
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, e)
		} else {
			files = append(files, e)
		}
	}
	if len(files) > 0 {
		return root, nil
	}
	if len(dirs) == 1 {
		return findContentDirHeuristic(filepath.Join(root, dirs[0].Name()))
	}
	return root, nil
}
