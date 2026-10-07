package instance

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// deadPID is a PID that is guaranteed not to be running.
const deadPID = 1 << 30

func newTestRegistry(t *testing.T) (*Registry, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "instances.json")
	reg := NewRegistry(path).WithWarn(func(format string, args ...any) {})
	return reg, path
}

func liveInstance(name string) Instance {
	return Instance{
		Name:    name,
		Browser: "chrome",
		Version: "120.0.6099.109",
		PID:     os.Getpid(),
	}
}

// REG-01: add then read back.
func TestRegistry_AddAndList(t *testing.T) {
	reg, _ := newTestRegistry(t)

	inst := liveInstance("bws-chrome-120")
	if err := reg.Add(inst); err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	list, err := reg.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("List() len = %d, want 1", len(list))
	}
	if list[0].Name != inst.Name || list[0].PID != inst.PID {
		t.Errorf("往返不一致: %+v", list[0])
	}
}

// REG-02: remove deletes an entry.
func TestRegistry_Remove(t *testing.T) {
	reg, _ := newTestRegistry(t)
	if err := reg.Add(liveInstance("bws-chrome-120")); err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	if _, err := reg.Remove("bws-chrome-120"); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}
	list, _ := reg.List()
	if len(list) != 0 {
		t.Errorf("Remove 后仍有 %d 个条目", len(list))
	}

	if _, err := reg.Remove("bws-chrome-120"); err == nil {
		t.Error("移除不存在的条目应返回错误")
	}
}

// REG-03: a corrupt file is treated as empty, without panicking.
func TestRegistry_CorruptFileTolerated(t *testing.T) {
	reg, path := newTestRegistry(t)

	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	var warned bool
	reg.WithWarn(func(format string, args ...any) { warned = true })

	list, err := reg.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(list) != 0 {
		t.Errorf("损坏文件应返回空注册表, got %d", len(list))
	}
	if !warned {
		t.Error("损坏文件应产生告警")
	}
}

// REG-04: entries whose process exited are pruned and reported.
func TestRegistry_ZombieCleanup(t *testing.T) {
	reg, path := newTestRegistry(t)

	stale := liveInstance("bws-chrome-stale")
	stale.PID = deadPID
	if err := reg.Save([]Instance{liveInstance("bws-chrome-live"), stale}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	var warned string
	reg.WithWarn(func(format string, args ...any) { warned = format })

	list, err := reg.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(list) != 1 || list[0].Name != "bws-chrome-live" {
		t.Fatalf("僵尸清理失败: %+v", list)
	}
	if !strings.Contains(warned, "失效实例") {
		t.Errorf("应告警清理失效实例, got %q", warned)
	}

	// The file must have been rewritten without the stale entry.
	raw, _ := os.ReadFile(path)
	if strings.Contains(string(raw), "bws-chrome-stale") {
		t.Errorf("僵尸条目未被持久化清理: %s", raw)
	}
}

// REG-05: concurrent adds do not lose or corrupt data.
func TestRegistry_ConcurrentAdd(t *testing.T) {
	reg, _ := newTestRegistry(t)

	const n = 12
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			inst := liveInstance(Name("chrome", "120.0.6099.109", "p"+string(rune('a'+i))))
			if err := reg.Add(inst); err != nil {
				t.Errorf("并发 Add 失败: %v", err)
			}
		}(i)
	}
	wg.Wait()

	raw, err := os.ReadFile(reg.Path())
	if err != nil {
		t.Fatalf("读取注册表失败: %v", err)
	}
	var file registryFile
	if err := json.Unmarshal(raw, &file); err != nil {
		t.Fatalf("注册表 JSON 损坏: %v", err)
	}
	if len(file.Instances) != n {
		t.Errorf("并发写入后条目数 = %d, want %d", len(file.Instances), n)
	}
}

// REG-06: canonical instance naming.
func TestRegistry_Name(t *testing.T) {
	cases := []struct {
		browser, ver, profile, want string
	}{
		{"chrome", "120.0.6099.109", "test-01", "bws-chrome-120-test-01"},
		{"chrome", "120.0.6099.109", "", "bws-chrome-120"},
		{"chrome", "120.0.6099.109", "default", "bws-chrome-120"},
		{"firefox", "121.0", "", "bws-firefox-121"},
	}
	for _, c := range cases {
		if got := Name(c.browser, c.ver, c.profile); got != c.want {
			t.Errorf("Name(%q,%q,%q) = %q, want %q", c.browser, c.ver, c.profile, got, c.want)
		}
	}
}

// REG-07: a missing file yields an empty registry.
func TestRegistry_EmptyWhenMissing(t *testing.T) {
	reg, _ := newTestRegistry(t)

	list, err := reg.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(list) != 0 {
		t.Errorf("空注册表 len = %d, want 0", len(list))
	}
}

// REG-08: Stop removes a residual entry whose process already exited.
func TestRegistry_StopAlreadyExited(t *testing.T) {
	reg, _ := newTestRegistry(t)

	stale := liveInstance("bws-chrome-120")
	stale.PID = deadPID
	if err := reg.Save([]Instance{stale}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	_, alreadyExited, err := reg.Stop("bws-chrome-120")
	if err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	if !alreadyExited {
		t.Error("进程已退出时应返回 alreadyExited=true")
	}

	list, _ := reg.List()
	if len(list) != 0 {
		t.Errorf("Stop 后条目应被移除, got %d", len(list))
	}
}

// REG-09: Add rejects a duplicate live instance name.
func TestRegistry_AddDuplicate(t *testing.T) {
	reg, _ := newTestRegistry(t)
	inst := liveInstance("bws-chrome-120")
	if err := reg.Add(inst); err != nil {
		t.Fatalf("首次 Add error = %v", err)
	}
	if err := reg.Add(inst); err == nil {
		t.Error("重复名称应返回错误")
	}
}

// REG-10: Get returns a live instance by name and errors on a missing name.
func TestRegistry_Get(t *testing.T) {
	reg, _ := newTestRegistry(t)
	if err := reg.Add(liveInstance("bws-chrome-120")); err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	got, err := reg.Get("bws-chrome-120")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Name != "bws-chrome-120" || got.Browser != "chrome" {
		t.Errorf("Get() = %+v", got)
	}

	if _, err := reg.Get("bws-chrome-999"); err == nil {
		t.Error("Get 不存在的实例应返回错误")
	}
}

// REG-11: Get prunes a stale entry so it is no longer returned.
func TestRegistry_GetPrunesStale(t *testing.T) {
	reg, _ := newTestRegistry(t)

	stale := liveInstance("bws-chrome-stale")
	stale.PID = deadPID
	if err := reg.Save([]Instance{stale}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	reg.WithWarn(func(format string, args ...any) {})

	if _, err := reg.Get("bws-chrome-stale"); err == nil {
		t.Error("已退出进程的实例不应被 Get 返回")
	}
}

// REG-12: Kill guards against invalid PIDs before touching the OS.
func TestKill_InvalidPID(t *testing.T) {
	for _, pid := range []int{0, -1} {
		if err := Kill(pid); err == nil {
			t.Errorf("Kill(%d) 应返回错误", pid)
		}
	}
}