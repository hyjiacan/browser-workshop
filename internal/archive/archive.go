// Package archive provides unified archive extraction using pure Go.
// Supports .zip, .tar.gz, .tar.bz2, .tar.xz, .tar.zst, .7z formats.
package archive

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/bzip2"
	"compress/gzip"
	"encoding/binary"
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

// safeFileMode masks out setuid/setgid/sticky bits from file modes.
func safeFileMode(mode os.FileMode) os.FileMode {
	return mode & 0o777
}

// isPathSafe 检查解压条目的路径是否在目标目录内，防止路径遍历攻击。
// 当 entryPath 解析后位于 destDir 之外时返回 false。
func isPathSafe(destDir, entryPath string) bool {
	cleanDest := filepath.Clean(destDir) + string(os.PathSeparator)
	return strings.HasPrefix(filepath.Clean(filepath.Join(destDir, entryPath)), cleanDest)
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
// It first checks the file extension, then falls back to magic byte detection.
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

	// Fallback to magic byte detection
	return detectFormatByMagic(path)
}

// magicBytes maps file format signatures to their extensions.
var magicBytes = []struct {
	magic []byte
	ext   string
}{
	{[]byte("PK\x03\x04"), ".zip"},
	{[]byte("7z\xBC\xAF\x27\x1C"), ".7z"},
	{[]byte{0x1F, 0x8B, 0x08}, ".gz"},
	{[]byte("BZh"), ".bz2"},
	{[]byte{0xFD, '7', 'z', 'X', 'Z', 0x00}, ".xz"},
	{[]byte{0x28, 0xB5, 0x2F, 0xFD}, ".zst"},
	{[]byte("MSCF"), ".cab"},
	{[]byte("MZ"), ".exe"},
}

// detectFormatByMagic reads the file header to detect the archive format.
func detectFormatByMagic(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()

	header := make([]byte, 16)
	n, err := f.Read(header)
	if err != nil || n < 4 {
		return ""
	}

	for _, m := range magicBytes {
		if n >= len(m.magic) {
			match := true
			for i, b := range m.magic {
				if header[i] != b {
					match = false
					break
				}
			}
			if match {
				return m.ext
			}
		}
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

	case format == ".exe":
		return extractExe(srcPath, destDir)

	case format == ".dmg", format == ".cab":
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

// extractExe extracts a .exe file by trying multiple methods in order:
//  1. Zip-based self-extractor (check magic bytes PK\x03\x04)
//  2. Embedded 7z archive (NSIS installers like Firefox embed a complete 7z archive)
//  3. Running the installer with -ExtractDir flag (Firefox NSIS installer)
//
// This handles common browser installer formats:
//   - Firefox "Firefox Setup X.exe" — NSIS installer with embedded 7z archive
//   - Chrome self-extracting zip — zip-based SFX
func extractExe(srcPath, destDir string) error {
	// Method 1: Try zip-based self-extractor
	if isZipFile(srcPath) {
		return extractZip(srcPath, destDir)
	}

	// Method 2: Try to find and extract embedded 7z archive (NSIS installers)
	cleanDirContents(destDir)
	if err := extractEmbedded7z(srcPath, destDir); err == nil {
		return nil
	}

	// Method 3: Try running the installer with -ExtractDir (Firefox NSIS)
	cleanDirContents(destDir)
	if err := extractExeViaInstaller(srcPath, destDir); err == nil {
		return nil
	}

	return fmt.Errorf("无法提取 .exe 文件：尝试了 zip、7z 嵌入提取、安装器提取，均失败")
}

// cleanDirContents removes all files and subdirectories within dir,
// but preserves the directory itself. Used to clean up between
// extraction attempts so partial results don't cause false positives.
// Errors are logged but not returned, as this is best-effort cleanup.
func cleanDirContents(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		path := filepath.Join(dir, entry.Name())
		if err := os.RemoveAll(path); err != nil {
			// Log but continue — this is cleanup between extraction attempts
			fmt.Fprintf(os.Stderr, "警告: 清理 %s 时出错: %v\n", path, err)
		}
	}
}

// extractEmbedded7z searches for a 7z archive embedded within a file
// (e.g., within an NSIS installer like Firefox) and extracts its contents.
//
// Firefox's NSIS installer has the following structure:
//   [NSIS header] [7z archive] [NSIS footer]
//
// The 7z archive is a complete, valid 7z file. We locate it by searching
// for the 7z signature (37 7A BC AF 27 1C), then parse the signature header
// to calculate the exact archive size, and extract just the 7z portion.
func extractEmbedded7z(srcPath, destDir string) error {
	data, err := os.ReadFile(srcPath)
	if err != nil {
		return err
	}

	// 7z signature: 37 7A BC AF 27 1C
	signature := []byte{0x37, 0x7A, 0xBC, 0xAF, 0x27, 0x1C}

	// Find all occurrences of the 7z signature (there may be false positives)
	searchStart := 0
	for {
		idx := bytes.Index(data[searchStart:], signature)
		if idx < 0 {
			break
		}
		offset := searchStart + idx

		// Try to extract from this offset
		if err := tryExtract7zAtOffset(data, offset, destDir); err == nil {
			return nil
		}

		// Move past this occurrence and continue searching
		searchStart = offset + len(signature)
	}

	return fmt.Errorf("未找到可提取的嵌入 7z 归档")
}

// tryExtract7zAtOffset attempts to extract a 7z archive starting at the given
// offset within data. It parses the 7z signature header to determine the exact
// archive size, writes the 7z portion to a temp file, and extracts it.
func tryExtract7zAtOffset(data []byte, offset int, destDir string) error {
	// 7z signature header is 32 bytes:
	//   0-5:   Signature (6 bytes)
	//   6:     Major version (1 byte)
	//   7:     Minor version (1 byte)
	//   8-11:  StartHeaderCRC (4 bytes)
	//   12-19: NextHeaderOffset (8 bytes, little-endian uint64)
	//   20-27: NextHeaderSize (8 bytes, little-endian uint64)
	//   28-31: NextHeaderCRC (4 bytes)
	const sigHeaderSize = 32

	if offset+sigHeaderSize > len(data) {
		return fmt.Errorf("7z 签名头不完整")
	}

	header := data[offset:]
	nextHeaderOffset := binary.LittleEndian.Uint64(header[12:20])
	nextHeaderSize := binary.LittleEndian.Uint64(header[20:28])

	// Total 7z archive size = signature header + packed data + next header
	archiveSize := uint64(sigHeaderSize) + nextHeaderOffset + nextHeaderSize
	endOffset := offset + int(archiveSize)
	if endOffset > len(data) {
		// Calculated size exceeds file — use file end as fallback
		endOffset = len(data)
	}

	// Write the 7z portion to a temp file
	tmpFile, err := os.CreateTemp("", "bws-embedded-*.7z")
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := tmpFile.Write(data[offset:endOffset]); err != nil {
		tmpFile.Close()
		return err
	}
	tmpFile.Close()

	// Try to extract using the 7z library
	return extract7z(tmpPath, destDir)
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
		// Reject symlinks to prevent path traversal via symlink targets
		if f.FileInfo().Mode()&os.ModeSymlink != 0 {
			continue
		}
		fpath := filepath.Join(destDir, f.Name)
		if !isPathSafe(destDir, f.Name) {
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
		dstFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, safeFileMode(f.FileInfo().Mode()))
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
		// Reject symlinks to prevent path traversal via symlink targets
		if f.Mode()&os.ModeSymlink != 0 {
			continue
		}
		fpath := filepath.Join(destDir, f.Name)
		if !isPathSafe(destDir, f.Name) {
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

		dstFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, safeFileMode(f.Mode()))
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
		if !isPathSafe(destDir, header.Name) {
			return fmt.Errorf("非法路径: %s", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeSymlink, tar.TypeLink:
			// Reject symlinks and hard links to prevent path traversal
			return fmt.Errorf("拒绝提取符号链接/硬链接: %s", header.Name)
		case tar.TypeDir:
			if err := os.MkdirAll(fpath, safeFileMode(os.FileMode(header.Mode))); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(fpath), 0o755); err != nil {
				return err
			}
			dstFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, safeFileMode(os.FileMode(header.Mode)))
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
