package help

import (
	"embed"
	"fmt"
	"sort"
	"strings"
)

//go:embed files/*.txt
var helpFiles embed.FS

// Topic represents a help topic.
type Topic struct {
	Name        string
	Description string
	Filename    string
}

// Topics returns a list of all available help topics.
func Topics() []Topic {
	entries, err := helpFiles.ReadDir("files")
	if err != nil {
		return nil
	}

	var topics []Topic
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ".txt")
		desc := topicDescription(name)
		topics = append(topics, Topic{
			Name:        name,
			Description: desc,
			Filename:    entry.Name(),
		})
	}

	sort.Slice(topics, func(i, j int) bool {
		// main always first
		if topics[i].Name == "main" {
			return true
		}
		if topics[j].Name == "main" {
			return false
		}
		return topics[i].Name < topics[j].Name
	})

	return topics
}

// Get returns the content of a help topic by name.
// Returns an error if the topic is not found.
func Get(topic string) (string, error) {
	// Try exact match first
	filename := fmt.Sprintf("files/%s.txt", topic)
	content, err := helpFiles.ReadFile(filename)
	if err == nil {
		return string(content), nil
	}

	// Try matching with common variations
	for _, t := range Topics() {
		lower := strings.ToLower(t.Name)
		if lower == strings.ToLower(topic) {
			content, err := helpFiles.ReadFile(fmt.Sprintf("files/%s", t.Filename))
			if err != nil {
				return "", err
			}
			return string(content), nil
		}
	}

	return "", fmt.Errorf("未找到帮助主题: %s\n\n可用主题: %s", topic, availableTopics())
}

// availableTopics returns a formatted list of available help topics.
func availableTopics() string {
	topics := Topics()
	var names []string
	for _, t := range topics {
		names = append(names, t.Name)
	}
	return strings.Join(names, ", ")
}

// topicDescription returns a short description for a help topic.
func topicDescription(name string) string {
	descriptions := map[string]string{
		"main":      "主帮助",
		"ls":        "版本列表查询",
		"install":   "下载并安装浏览器版本",
		"run":       "运行浏览器",
		"shortcut":  "快捷方式管理",
		"repo":      "仓库管理",
		"serve":     "HTTP 离线下载源服务",
		"config":    "配置管理",
		"profile":   "Profile 管理",
		"plugin":    "插件管理",
		"alias":     "版本别名管理",
		"cache":     "下载缓存管理",
		"doctor":    "系统健康检查",
		"download":  "下载浏览器版本",
		"uninstall": "卸载浏览器版本",
		"use":       "设置默认版本",
		"info":      "显示版本详细信息",
		"sources":   "数据源说明",
		"concepts":  "核心概念",
		"faq":       "常见问题",
	}
	if desc, ok := descriptions[name]; ok {
		return desc
	}
	return ""
}