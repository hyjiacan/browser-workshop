// Package i18n provides lightweight internationalization for the bws CLI.
//
// 翻译来源优先级（从高到低）：
//  1. 外部翻译文件 ~/.bws/i18n/<lang>.json（用户自定义，覆盖内置）
//  2. 内置嵌入语言包 langs/<lang>.json（通过 embed 编译进二进制）
//  3. 内置 Go 代码 fallback（JSON 加载失败时）
//
// 添加新语言：在 langs/ 目录下新建 <lang>.json 即可，无需修改任何 Go 代码。
//
// 使用:
//
//	i18n.Init("")                 // 自动检测语言，默认中文
//	i18n.T("root.commands")       // 获取翻译
//	i18n.Tfmt("root.usage", "bws") // 带参数的翻译
package i18n

import (
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

//go:embed langs/*.json
var langFS embed.FS

var (
	mu     sync.RWMutex
	loaded map[string]string
)

// Init initializes the translation system.
// lang can be "zh", "en" or empty (auto-detect).
// externalDir is the directory for user translation overrides (e.g. <dataDir>/i18n).
func Init(lang string, externalDir string) {
	mu.Lock()
	defer mu.Unlock()

	if lang == "" {
		lang = detectLang()
	}

	// 1. 尝试从 embed 加载内置翻译
	loaded = loadEmbed(lang)
	if loaded == nil {
		// 2. embed 失败，fallback 到内置代码
		loaded = builtinZh()
		if lang == "en" {
			for k, v := range builtinEn() {
				loaded[k] = v
			}
		}
	}

	// 3. 外部文件覆盖（最高优先级）
	loadExternal(lang, externalDir)
}

// T returns the translated string for the given key.
func T(key string) string {
	mu.RLock()
	defer mu.RUnlock()
	if v, ok := loaded[key]; ok {
		return v
	}
	return key
}

// Tfmt returns the translated string formatted with the given arguments.
func Tfmt(key string, args ...interface{}) string {
	return fmt.Sprintf(T(key), args...)
}

// detectLang detects the system language from environment variables.
func detectLang() string {
	if v := os.Getenv("LANG"); v != "" && len(v) >= 2 && v[:2] == "en" {
		return "en"
	}
	if v := os.Getenv("LANGUAGE"); v != "" && len(v) >= 2 && v[:2] == "en" {
		return "en"
	}
	return "zh"
}

// loadEmbed loads a built-in translation from the embedded langs/ directory.
func loadEmbed(lang string) map[string]string {
	data, err := langFS.ReadFile("langs/" + lang + ".json")
	if err != nil {
		// 目标语言不存在，尝试中文
		if lang != "zh" {
			data, err = langFS.ReadFile("langs/zh.json")
			if err != nil {
				return nil
			}
		} else {
			return nil
		}
	}

	var m map[string]string
	if err := json.Unmarshal(data, &m); err != nil {
		return nil
	}
	return m
}

// loadExternal loads translation overrides from an external JSON file.
func loadExternal(lang string, externalDir string) {
	if externalDir == "" {
		return
	}

	path := filepath.Join(externalDir, lang+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}

	var external map[string]string
	if err := json.Unmarshal(data, &external); err != nil {
		return
	}

	for k, v := range external {
		loaded[k] = v
	}
}

// builtinZh is the fallback Chinese translation (used when embed fails).
func builtinZh() map[string]string {
	return map[string]string{
		"root.description": "浏览器版本管理工具",
		"root.usage":        "用法:\n  %s <命令> [选项]",
		"root.commands":     "可用命令:",
		"root.flags":        "选项:",
		"root.flag_help":    "显示帮助",
		"root.flag_version": "显示版本",
		"root.help_line1":   "使用 '%s <命令> --help' 查看命令详情。",
		"root.help_line2":   "使用 '%s help <主题>' 查看详细帮助。",

		"cmd.usage":       "用法:",
		"cmd.examples":    "示例:",
		"cmd.subcommands": "子命令:",
		"cmd.flags":       "选项:",
		"cmd.default":     "默认",
		"cmd.help_line":   "使用 '%s help %s' 查看详细帮助。",

		"error.unknown_command":    "未知命令: %s",
		"error.unknown_subcommand": "\"%s\" 没有子命令 \"%s\"",
		"error.typo_suggestion":    "你是不是想用 \"%s\"? (相似度: %d%%)",
		"error.unknown_option":     "未知选项: %s",

		"confirm.prompt": "%s [是/否]: ",
	}
}

// builtinEn is the fallback English translation (used when embed fails).
func builtinEn() map[string]string {
	return map[string]string{
		"root.description": "Browser Version Manager",
		"root.usage":        "Usage:\n  %s <command> [options]",
		"root.commands":     "Available commands:",
		"root.flags":        "Flags:",
		"root.flag_help":    "Show help",
		"root.flag_version": "Show version",
		"root.help_line1":   "Use '%s <command> --help' for more information about a command.",
		"root.help_line2":   "Use '%s help <topic>' for detailed help.",

		"cmd.usage":       "Usage:",
		"cmd.examples":    "Examples:",
		"cmd.subcommands": "Subcommands:",
		"cmd.flags":       "Flags:",
		"cmd.default":     "default",
		"cmd.help_line":   "Use '%s help %s' for detailed help.",

		"error.unknown_command":    "Unknown command: %s",
		"error.unknown_subcommand": "\"%s\" has no subcommand \"%s\"",
		"error.typo_suggestion":    "Did you mean \"%s\"? (similarity: %d%%)",
		"error.unknown_option":     "Unknown option: %s",

		"confirm.prompt": "%s [y/N]: ",
	}
}