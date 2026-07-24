package cli

import (
	"fmt"
	"io"
	"math"
	"strings"

	"github.com/bws/bws/internal/i18n"
)

// levenshteinDistance 计算两个字符串之间的编辑距离（Levenshtein Distance）
// 基于动态规划实现，时间复杂度 O(mn)，空间复杂度 O(min(m,n))
func levenshteinDistance(a, b string) int {
	aRunes := []rune(a)
	bRunes := []rune(b)
	aLen := len(aRunes)
	bLen := len(bRunes)

	// 确保 a 是较短的字符串以优化空间
	if aLen > bLen {
		aRunes, bRunes = bRunes, aRunes
		aLen, bLen = bLen, aLen
	}

	// 使用两行滚动数组
	prev := make([]int, bLen+1)
	curr := make([]int, bLen+1)

	for j := 0; j <= bLen; j++ {
		prev[j] = j
	}

	for i := 1; i <= aLen; i++ {
		curr[0] = i
		for j := 1; j <= bLen; j++ {
			cost := 1
			if aRunes[i-1] == bRunes[j-1] {
				cost = 0
			}
			// 取删除、插入、替换中的最小值
			curr[j] = minInt(prev[j]+1, curr[j-1]+1, prev[j-1]+cost)
		}
		prev, curr = curr, prev
	}

	return prev[bLen]
}

// minInt 返回多个整数中的最小值
func minInt(vals ...int) int {
	m := math.MaxInt32
	for _, v := range vals {
		if v < m {
			m = v
		}
	}
	return m
}

// similarityScore 计算相似度分数（0.0 ~ 1.0，越高越相似）
// 基于编辑距离和字符串长度进行归一化
func similarityScore(input, candidate string) float64 {
	inputRunes := []rune(input)
	candidateRunes := []rune(candidate)
	dist := levenshteinDistance(input, candidate)
	maxLen := len(inputRunes)
	if len(candidateRunes) > maxLen {
		maxLen = len(candidateRunes)
	}
	if maxLen == 0 {
		return 1.0
	}
	return 1.0 - float64(dist)/float64(maxLen)
}

// suggestCommand 从候选命令列表中找出最相似的命令
// 返回推荐的命令名和相似度分数
func suggestCommand(input string, candidates []string) (string, float64) {
	if len(candidates) == 0 {
		return "", 0
	}

	// 1. 精确匹配？直接返回
	lowerInput := strings.ToLower(input)
	for _, c := range candidates {
		if strings.ToLower(c) == lowerInput {
			return c, 1.0
		}
	}

	// 2. 前缀匹配加权
	bestScore := 0.0
	bestMatch := ""
	for _, c := range candidates {
		// 基础编辑距离相似度
		score := similarityScore(input, c)

		// 前缀匹配加分
		lowerC := strings.ToLower(c)
		if strings.HasPrefix(lowerC, lowerInput) {
			score = math.Max(score, 0.6)
		}

		// 公共前缀加分
		commonPrefix := commonPrefixLen(lowerInput, lowerC)
		if commonPrefix > 0 && float64(commonPrefix) >= float64(len(lowerInput))*0.5 {
			score += 0.1
		}

		// Damerau-Levenshtein 相邻字符交换优化：如果仅差一个相邻交换
		if len(input) == len(c) && levenshteinDistance(input, c) == 2 {
			// 检查是否仅交换相邻字符
			if isAdjacentSwap(input, c) {
				score = math.Max(score, 0.85)
			}
		}

		if score > bestScore {
			bestScore = score
			bestMatch = c
		}
	}

	return bestMatch, bestScore
}

// commonPrefixLen 计算两个字符串的公共前缀长度
func commonPrefixLen(a, b string) int {
	maxLen := len(a)
	if len(b) < maxLen {
		maxLen = len(b)
	}
	for i := 0; i < maxLen; i++ {
		if a[i] != b[i] {
			return i
		}
	}
	return maxLen
}

// isAdjacentSwap 检查两个字符串是否仅差一个相邻字符交换
func isAdjacentSwap(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	swaps := 0
	for i := 0; i < len(a)-1; i++ {
		if a[i] != b[i] {
			if a[i] == b[i+1] && a[i+1] == b[i] {
				swaps++
				if swaps > 1 {
					return false
				}
				i++ // 跳过已交换的字符
			} else {
				return false
			}
		}
	}
	return swaps == 1
}

// collectCommandCandidates 从 Command 树中收集所有命令名和别名
func collectCommandCandidates(cmd *Command) []string {
	var names []string
	seen := make(map[string]bool)

	for _, sub := range cmd.SubCommands {
		if !seen[sub.Name] {
			seen[sub.Name] = true
			names = append(names, sub.Name)
		}
		for _, alias := range sub.Aliases {
			if !seen[alias] {
				seen[alias] = true
				names = append(names, alias)
			}
		}
	}

	return names
}

// printTypoSuggestion 在输出中打印 typo 建议
func printTypoSuggestion(w io.Writer, input string, candidates []string) {
	suggestion, score := suggestCommand(input, candidates)
	// 阈值：相似度低于 0.35 不推荐
	if suggestion != "" && score >= 0.35 {
		fmt.Fprintf(w, "\n%s\n", i18n.Tfmt("error.typo_suggestion", suggestion, int(score*100)))
	}
}

// findCommandWithTypo 在命令树中查找命令，找不到时返回当前层级可用的候选列表
// 返回值: 找到的命令, 剩余参数, 当前层级候选列表, 是否精确匹配
func findCommandWithTypo(cmd *Command, args []string) (*Command, []string, []string, bool) {
	if len(args) == 0 {
		return cmd, args, nil, true
	}

	name := args[0]
	candidates := collectCommandCandidates(cmd)

	for _, sub := range cmd.SubCommands {
		if sub.Name == name {
			// 继续在子命令中递归查找
			found, remaining, _, subMatched := findCommandWithTypo(sub, args[1:])
			// 如果子命令匹配失败，把子命令层的候选列表往上传（带上当前命令前缀作为上下文）
			if !subMatched {
				// 收集子命令的候选（作为更精确的提示）
				subCmdCandidates := collectCommandCandidates(sub)
				return found, remaining, subCmdCandidates, false
			}
			return found, remaining, nil, true
		}
		for _, alias := range sub.Aliases {
			if alias == name {
				found, remaining, _, subMatched := findCommandWithTypo(sub, args[1:])
				if !subMatched {
					subCmdCandidates := collectCommandCandidates(sub)
					return found, remaining, subCmdCandidates, false
				}
				return found, remaining, nil, true
			}
		}
	}

	// 未找到匹配，返回当前 cmd 和候选列表
	return cmd, args, candidates, false
}