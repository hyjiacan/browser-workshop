package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func newTestOutput(jsonMode bool) (*Output, *bytes.Buffer, *bytes.Buffer) {
	var stdout, stderr bytes.Buffer
	ctx := &Context{Stdout: &stdout, Stderr: &stderr}
	return NewOutput(ctx, "demo", jsonMode), &stdout, &stderr
}

// OUT-01: JSON success envelope.
func TestOutput_JSONSuccessEnvelope(t *testing.T) {
	out, stdout, _ := newTestOutput(true)

	if err := out.Success(map[string]any{"name": "bws"}); err != nil {
		t.Fatalf("Success() error = %v", err)
	}

	var env map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("输出不是合法 JSON: %v\n%s", err, stdout.String())
	}
	if env["ok"] != true {
		t.Errorf("ok = %v, want true", env["ok"])
	}
	if env["command"] != "demo" {
		t.Errorf("command = %v, want demo", env["command"])
	}
	if env["appname"] != "bws" {
		t.Errorf("appname = %v, want bws", env["appname"])
	}
	if v, _ := env["version"].(string); v == "" {
		t.Error("version 不应为空")
	}
	ts, _ := env["timestamp"].(string)
	if ts == "" {
		t.Fatal("timestamp 不应为空")
	}
	if _, perr := time.Parse(time.RFC3339, ts); perr != nil {
		t.Errorf("timestamp 不是 RFC3339: %v (%q)", perr, ts)
	}
	data, _ := env["data"].(map[string]any)
	if data["name"] != "bws" {
		t.Errorf("data.name = %v, want bws", data["name"])
	}
	if strings.Contains(stdout.String(), "  \"ok\"") == false {
		t.Errorf("期望缩进 2 空格的 JSON: %s", stdout.String())
	}
}

// OUT-02: JSON error envelope.
func TestOutput_JSONErrorEnvelope(t *testing.T) {
	out, stdout, _ := newTestOutput(true)

	err := out.Error(ErrCodeNotFound, "找不到实例")
	if err == nil {
		t.Fatal("Error() 应返回非 nil error")
	}

	var env struct {
		OK      bool   `json:"ok"`
		Command string `json:"command"`
		Error   struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if uerr := json.Unmarshal(stdout.Bytes(), &env); uerr != nil {
		t.Fatalf("输出不是合法 JSON: %v", uerr)
	}
	if env.OK {
		t.Error("ok 应为 false")
	}
	if env.Error.Code != ErrCodeNotFound {
		t.Errorf("error.code = %q, want %q", env.Error.Code, ErrCodeNotFound)
	}
	if env.Error.Message != "找不到实例" {
		t.Errorf("error.message = %q", env.Error.Message)
	}
}

// OUT-03: non-JSON object alignment.
func TestOutput_ObjectAlignment(t *testing.T) {
	out, stdout, _ := newTestOutput(false)

	out.Object([]Field{{"a", "1"}, {"longkey", "2"}})

	want := "a:       1\nlongkey: 2\n"
	if stdout.String() != want {
		t.Errorf("Object 输出:\n%q\nwant:\n%q", stdout.String(), want)
	}
}

// OUT-04: grouped rendering with blank line between groups.
func TestOutput_Groups(t *testing.T) {
	out, stdout, _ := newTestOutput(false)

	out.Groups([]Group{
		{Title: "组一", Fields: []Field{{"k", "v"}, {"long", "w"}}},
		{Title: "组二", Fields: []Field{{"k2", "v2"}}},
	})

	got := stdout.String()
	// 组内按最长键对齐，组间空行；组间互不影响。
	want := "组一\n  k:    v\n  long: w\n\n组二\n  k2: v2\n"
	if got != want {
		t.Errorf("Groups 输出:\n%q\nwant:\n%q", got, want)
	}
}

// OUT-05: table alignment with CJK text.
func TestOutput_TableCJK(t *testing.T) {
	out, stdout, _ := newTestOutput(false)

	out.Table([]string{"名称", "值"}, [][]string{{"中文", "x"}, {"ab", "y"}})

	lines := strings.Split(strings.TrimRight(stdout.String(), "\n"), "\n")
	if len(lines) != 4 {
		t.Fatalf("行数 = %d, want 4\n%s", len(lines), stdout.String())
	}
	// "名称" and "中文" are width 4; both rows must align the second column.
	if !strings.HasPrefix(lines[2], "中文") || !strings.HasPrefix(lines[3], "ab  ") {
		t.Errorf("CJK 对齐不正确:\n%s", stdout.String())
	}
}

// OUT-06: metadata always goes to stderr.
func TestOutput_MetaToStderr(t *testing.T) {
	out, stdout, stderr := newTestOutput(false)

	out.Meta("提示 %d", 1)

	if stdout.Len() != 0 {
		t.Errorf("stdout 应为空, got %q", stdout.String())
	}
	if stderr.String() != "提示 1\n" {
		t.Errorf("stderr = %q", stderr.String())
	}
}

// OUT-07: Emit splits JSON vs human output.
func TestOutput_EmitSplit(t *testing.T) {
	// JSON mode: human renderer must not run.
	out, stdout, _ := newTestOutput(true)
	humanRan := false
	if err := out.Emit(map[string]any{"x": 1}, func() { humanRan = true }); err != nil {
		t.Fatalf("Emit error = %v", err)
	}
	if humanRan {
		t.Error("JSON 模式不应执行 human 渲染")
	}
	if !strings.Contains(stdout.String(), `"x": 1`) {
		t.Errorf("JSON 模式输出缺失 data: %s", stdout.String())
	}

	// Human mode: envelope must not be written.
	out2, stdout2, _ := newTestOutput(false)
	humanRan = false
	if err := out2.Emit(map[string]any{"x": 1}, func() { humanRan = true }); err != nil {
		t.Fatalf("Emit error = %v", err)
	}
	if !humanRan {
		t.Error("非 JSON 模式应执行 human 渲染")
	}
	if stdout2.Len() != 0 {
		t.Errorf("非 JSON 模式 stdout 应为空, got %q", stdout2.String())
	}
}

// OUT-08: null endpoint fields are preserved (not omitted).
func TestOutput_NullFieldsPreserved(t *testing.T) {
	out, stdout, _ := newTestOutput(true)

	var cdp *string
	if err := out.Success(struct {
		CDP *string `json:"cdp"`
	}{CDP: cdp}); err != nil {
		t.Fatalf("Success error = %v", err)
	}

	if !strings.Contains(stdout.String(), `"cdp": null`) {
		t.Errorf("null 字段应保留: %s", stdout.String())
	}
}

// OUT-09: the command field carries the raw invocation (options/arguments) when
// the context provides one, falling back to the canonical name otherwise.
func TestOutput_CommandCarriesInvocation(t *testing.T) {
	var stdout, stderr bytes.Buffer
	ctx := &Context{Stdout: &stdout, Stderr: &stderr, Invocation: "run chrome@120 --automation --json"}
	out := NewOutput(ctx, "run", true)

	if err := out.Success(map[string]any{"pid": 1}); err != nil {
		t.Fatalf("Success error = %v", err)
	}

	var env map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("输出不是合法 JSON: %v\n%s", err, stdout.String())
	}
	if want := "run chrome@120 --automation --json"; env["command"] != want {
		t.Errorf("command = %v, want %q", env["command"], want)
	}
}
