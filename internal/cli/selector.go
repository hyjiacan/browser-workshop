// Package cli - Interactive version selection strategies.
//
// Strategy pattern: different selection methods can be injected into the
// version resolution pipeline depending on the execution environment.
//
//	TTY (terminal)  → InteractiveSelector (arrow-key navigation via huh)
//	Pipe (non-TTY)  → AutoSelector (pick first), or NumberedSelector fallback
package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/x/term"
)

// ---------------------------------------------------------------
// Strategy Interface
// ---------------------------------------------------------------

// SelectionStrategy decides which version to pick from a list of choices.
// Returns (versionChoice{}, true) if the user explicitly cancelled.
type SelectionStrategy interface {
	Select(ctx *Context, choices []versionChoice) (versionChoice, bool)
}

// DetectSelector returns the best-selector for the current terminal environment.
func DetectSelector() SelectionStrategy {
	if term.IsTerminal(os.Stdin.Fd()) {
		return &interactiveSelector{}
	}
	// Non-TTY: auto-select the first choice; caller must handle multi-choice.
	return &autoSelector{}
}

// ---------------------------------------------------------------
// InteractiveSelector (arrow-key navigation via huh)
// ---------------------------------------------------------------

type interactiveSelector struct{}

func (s *interactiveSelector) Select(ctx *Context, choices []versionChoice) (versionChoice, bool) {
	if len(choices) == 0 {
		return versionChoice{}, true
	}
	if len(choices) == 1 {
		return choices[0], false
	}

	// Build huh options with informative labels.
	opts := make([]huh.Option[versionChoice], len(choices))
	for i, c := range choices {
		label := formatChoiceLabel(c)
		opts[i] = huh.NewOption(label, c)
	}

	var selected versionChoice
	err := huh.NewSelect[versionChoice]().
		Title("选择要启动的版本").
		Description("↑↓ 移动  Enter 确认  q/Ctrl+C 取消").
		Options(opts...).
		Value(&selected).
		WithHeight(min(12, len(choices)+6)).
		Run()
	if err != nil {
		return versionChoice{}, true
	}
	return selected, false
}

// formatChoiceLabel builds a padded label: "141.0     [stable]   (已安装)"
func formatChoiceLabel(c versionChoice) string {
	status := ""
	if c.Installed {
		status = " (已安装)"
	}
	ch := c.Channel
	if len(ch) > 10 {
		ch = ch[:10]
	}
	return fmt.Sprintf("%-20s [%-10s]%s", c.Version, ch, status)
}

// ---------------------------------------------------------------
// AutoSelector (single choice, silent)
// ---------------------------------------------------------------

type autoSelector struct{}

func (s *autoSelector) Select(ctx *Context, choices []versionChoice) (versionChoice, bool) {
	if len(choices) == 0 {
		return versionChoice{}, true
	}
	return choices[0], false
}

// ---------------------------------------------------------------
// NumberedSelector (fallback prompt via stdin/stdout)
// ---------------------------------------------------------------

// numberedSelector is used when:
//   - stdin is not a TTY (e.g., piped input)
//   - BUT we still want to allow numbered input from the user
//
// It reads a single line from the provided reader.
type numberedSelector struct {
	reader io.Reader
	writer io.Writer
}

// NewNumberedSelector creates a fallback numbered-list selector.
func NewNumberedSelector(r io.Reader, w io.Writer) SelectionStrategy {
	return &numberedSelector{reader: r, writer: w}
}

func (s *numberedSelector) Select(ctx *Context, choices []versionChoice) (versionChoice, bool) {
	if len(choices) == 0 {
		return versionChoice{}, true
	}
	if len(choices) == 1 {
		return choices[0], false
	}

	// Build output
	buf := new(strings.Builder)
	fmt.Fprintf(buf, "\n发现 %d 个匹配版本:\n\n", len(choices))
	for i, c := range choices {
		tag := ""
		if c.Installed {
			tag = " (已安装)"
		}
		fmt.Fprintf(buf, "  %2d) %s  [%s]%s\n", i+1, c.Version, c.Channel, tag)
	}
	fmt.Fprintf(buf, "   0) 取消\n")
	fmt.Fprintf(buf, "\n请输入序号 (1-%d): ", len(choices))
	s.writer.Write([]byte(buf.String()))

	// Read single line
	var line string
	fmt.Fscanln(s.reader, &line)
	line = strings.TrimSpace(line)

	if line == "0" || line == "" {
		return versionChoice{}, true
	}

	n := 0
	fmt.Sscan(line, &n)
	if n < 1 || n > len(choices) {
		s.writer.Write([]byte("无效输入，已取消。\n"))
		return versionChoice{}, true
	}

	return choices[n-1], false
}
