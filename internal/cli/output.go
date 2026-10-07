package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// Error codes used in the JSON error envelope. They are stable, language-neutral
// identifiers so scripts can branch on them without parsing human messages.
const (
	ErrCodeNotFound        = "NOT_FOUND"
	ErrCodeInvalidArgument = "INVALID_ARGUMENT"
	ErrCodeUnsupported     = "UNSUPPORTED"
	ErrCodeConflict        = "CONFLICT"
	ErrCodeInternal        = "INTERNAL"
)

// Field is a single key/value pair rendered as an aligned "key: value" line.
type Field struct {
	Key   string
	Value string
}

// Group is a titled block of key/value fields.
type Group struct {
	Title  string
	Fields []Field
}

// jsonEnvelope is the stable top-level structure shared by every command's
// JSON output. The shape is a public contract: fields are only ever added,
// never renamed or removed.
type jsonEnvelope struct {
	OK      bool       `json:"ok"`
	Command string     `json:"command"`
	Data    any        `json:"data,omitempty"`
	Error   *jsonError `json:"error,omitempty"`
}

type jsonError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Output implements the unified output contract described in the automation
// integration design: every command renders either a stable JSON envelope or a
// human-readable form, and metadata always goes to stderr.
type Output struct {
	JSON    bool
	Command string
	Stdout  io.Writer
	Stderr  io.Writer
}

// NewOutput builds an Output bound to a command's context.
func NewOutput(ctx *Context, command string, jsonMode bool) *Output {
	return &Output{
		JSON:    jsonMode,
		Command: command,
		Stdout:  ctx.Stdout,
		Stderr:  ctx.Stderr,
	}
}

// Object renders an aligned single-object key/value block.
//
//	Instance:  bws-chrome-120
//	Binary:    C:\...\chrome.exe
//
// Keys are padded so that every colon and value starts at the same column.
func (o *Output) Object(fields []Field) {
	o.writeFields(fields, "")
}

// Groups renders titled groups separated by a blank line. Fields inside a group
// are indented and aligned to that group's longest key.
func (o *Output) Groups(groups []Group) {
	for i, g := range groups {
		if i > 0 {
			fmt.Fprintln(o.Stdout)
		}
		if g.Title != "" {
			fmt.Fprintln(o.Stdout, g.Title)
		}
		o.writeFields(g.Fields, "  ")
	}
}

// Table renders an aligned table (columns sized by display width so CJK text
// lines up in terminals).
func (o *Output) Table(headers []string, rows [][]string) {
	PrintTable(o.Stdout, headers, rows)
}

// Meta writes metadata (banners, hints, warnings) to stderr so that stdout
// stays a clean, pipeable payload.
func (o *Output) Meta(format string, args ...any) {
	fmt.Fprintf(o.Stderr, format+"\n", args...)
}

// Success writes the JSON success envelope. It is a no-op in non-JSON mode.
func (o *Output) Success(data any) error {
	if !o.JSON {
		return nil
	}
	return o.encode(jsonEnvelope{OK: true, Command: o.Command, Data: data})
}

// Error writes the JSON error envelope (JSON mode) and returns an error so the
// process exits non-zero. In non-JSON mode the caller's error propagation
// prints the message to stderr.
func (o *Output) Error(code, message string) error {
	if o.JSON {
		_ = o.encode(jsonEnvelope{
			OK:      false,
			Command: o.Command,
			Error:   &jsonError{Code: code, Message: message},
		})
	}
	return fmt.Errorf("%s", message)
}

// Emit is the common entry point: it writes the JSON envelope when JSON mode is
// enabled, otherwise invokes the human-readable renderer.
func (o *Output) Emit(data any, human func()) error {
	if o.JSON {
		return o.Success(data)
	}
	human()
	return nil
}

// writeFields renders key/value fields with the given line prefix, aligning all
// colons to the longest key in the block.
func (o *Output) writeFields(fields []Field, prefix string) {
	maxKey := 0
	for _, f := range fields {
		if w := displayWidth(f.Key); w > maxKey {
			maxKey = w
		}
	}
	// +1 accounts for the colon that follows each key.
	labelWidth := maxKey + 1
	for _, f := range fields {
		label := f.Key + ":"
		pad := labelWidth - displayWidth(label)
		if pad < 0 {
			pad = 0
		}
		fmt.Fprintf(o.Stdout, "%s%s%s %s\n", prefix, label, strings.Repeat(" ", pad), f.Value)
	}
}

func (o *Output) encode(env jsonEnvelope) error {
	data, err := json.MarshalIndent(env, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化 JSON 输出失败: %w", err)
	}
	_, err = fmt.Fprintln(o.Stdout, string(data))
	return err
}