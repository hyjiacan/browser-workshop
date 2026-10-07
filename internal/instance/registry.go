// Package instance manages the background-instance registry.
//
// The registry is the single source of truth for instances started with
// `bws run --daemon`: `ps` reads it, `stop` mutates it. It is stored as a
// single JSON file under the bws data directory (see design §4.5).
package instance

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	bmlog "github.com/bws/bws/internal/log"
	"github.com/bws/bws/internal/version"
)

// Instance describes a running browser instance tracked by the registry.
type Instance struct {
	Name       string    `json:"name"`
	Browser    string    `json:"browser"`
	Version    string    `json:"version"`
	Profile    string    `json:"profile"`
	PID        int       `json:"pid"`
	Binary     string    `json:"binary"`
	ProfileDir string    `json:"profileDir"`
	CDP        *string   `json:"cdp"`
	WebDriver  *string   `json:"webdriver"`
	DriverPID  int       `json:"driverPid"`
	StartedAt  time.Time `json:"startedAt"`
	Daemon     bool      `json:"daemon"`
}

// registryFile is the on-disk wrapper, kept as an object so future metadata can
// be added without breaking readers.
type registryFile struct {
	Instances []Instance `json:"instances"`
}

// Registry provides locked, corruption-tolerant access to the instance file.
type Registry struct {
	path string
	mu   sync.Mutex
	warn func(format string, args ...any)
}

// NewRegistry creates a registry backed by the given file path.
func NewRegistry(path string) *Registry {
	return &Registry{
		path: path,
		warn: func(format string, args ...any) { bmlog.Warn(format, args...) },
	}
}

// WithWarn overrides the warning sink (used by tests and embedders).
func (r *Registry) WithWarn(fn func(format string, args ...any)) *Registry {
	if fn != nil {
		r.warn = fn
	}
	return r
}

// Path returns the registry file path.
func (r *Registry) Path() string { return r.path }

// Name builds the canonical instance name: bws-<browser>-<major>[-<profile>].
func Name(browser, ver, profile string) string {
	name := "bws-" + browser
	if major := version.Major(ver); major > 0 {
		name += "-" + strconv.Itoa(major)
	} else if v := strings.TrimSpace(ver); v != "" {
		name += "-" + v
	}
	if p := strings.TrimSpace(profile); p != "" && p != "default" {
		name += "-" + p
	}
	return name
}

// Load reads the registry without pruning. A missing file yields an empty list.
// A corrupt file is treated as empty and reported through the warning sink.
func (r *Registry) Load() ([]Instance, error) {
	data, err := os.ReadFile(r.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return nil, nil
	}
	var file registryFile
	if err := json.Unmarshal(data, &file); err != nil {
		r.warn("实例注册表损坏，已按空注册表处理: %v", err)
		return nil, nil
	}
	return file.Instances, nil
}

// Save writes the registry atomically under an exclusive file lock.
func (r *Registry) Save(list []Instance) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.saveLocked(list)
}

func (r *Registry) saveLocked(list []Instance) error {
	return withFileLock(r.path+".lock", func() error {
		if err := os.MkdirAll(filepath.Dir(r.path), 0o755); err != nil {
			return err
		}
		data, err := json.MarshalIndent(registryFile{Instances: list}, "", "  ")
		if err != nil {
			return err
		}
		tmp := r.path + ".tmp"
		if err := os.WriteFile(tmp, data, 0o644); err != nil {
			return err
		}
		return os.Rename(tmp, r.path)
	})
}

// List returns live instances, pruning entries whose process has exited.
// Stale entries are removed from the registry and reported via the warning sink.
func (r *Registry) List() ([]Instance, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	list, err := r.Load()
	if err != nil {
		return nil, err
	}

	alive := make([]Instance, 0, len(list))
	stale := make([]string, 0)
	for _, inst := range list {
		if inst.PID > 0 && !ProcessAlive(inst.PID) {
			stale = append(stale, inst.Name)
			continue
		}
		alive = append(alive, inst)
	}

	if len(stale) > 0 {
		r.warn("已清理失效实例: %s", strings.Join(stale, ", "))
		if err := r.saveLocked(alive); err != nil {
			return nil, err
		}
	}
	return alive, nil
}

// Get returns a single live instance by name. Stale matching entries are pruned.
func (r *Registry) Get(name string) (*Instance, error) {
	list, err := r.List()
	if err != nil {
		return nil, err
	}
	for i := range list {
		if list[i].Name == name {
			return &list[i], nil
		}
	}
	return nil, fmt.Errorf("实例不存在: %s", name)
}

// Add inserts a new instance. It fails when the name is already taken.
func (r *Registry) Add(inst Instance) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	list, err := r.Load()
	if err != nil {
		return err
	}
	for _, existing := range list {
		if existing.Name == inst.Name && ProcessAlive(existing.PID) {
			return fmt.Errorf("实例 %s 已存在，请先执行 'bws stop %s' 或更换 Profile", inst.Name, inst.Name)
		}
	}
	// Drop any stale entry with the same name before inserting.
	filtered := list[:0]
	for _, existing := range list {
		if existing.Name != inst.Name {
			filtered = append(filtered, existing)
		}
	}
	list = append(filtered, inst)
	return r.saveLocked(list)
}

// Remove deletes an instance by name. It returns the removed instance, or an
// error when the name is not registered.
func (r *Registry) Remove(name string) (*Instance, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	list, err := r.Load()
	if err != nil {
		return nil, err
	}
	var removed *Instance
	kept := make([]Instance, 0, len(list))
	for i := range list {
		if list[i].Name == name {
			removed = &list[i]
			continue
		}
		kept = append(kept, list[i])
	}
	if removed == nil {
		return nil, fmt.Errorf("实例不存在: %s", name)
	}
	if err := r.saveLocked(kept); err != nil {
		return nil, err
	}
	return removed, nil
}

// Stop terminates an instance's browser and driver processes and removes it
// from the registry. The profile directory is intentionally left untouched.
//
// Unlike Get, Stop reads the raw registry (without pruning) so that a residual
// entry whose process already exited can still be reported and cleaned up.
func (r *Registry) Stop(name string) (*Instance, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	list, err := r.Load()
	if err != nil {
		return nil, false, err
	}

	var target *Instance
	kept := make([]Instance, 0, len(list))
	for i := range list {
		if list[i].Name == name {
			target = &list[i]
			continue
		}
		kept = append(kept, list[i])
	}
	if target == nil {
		return nil, false, fmt.Errorf("实例不存在: %s", name)
	}

	alreadyExited := target.PID <= 0 || !ProcessAlive(target.PID)
	if !alreadyExited {
		if err := Kill(target.PID); err != nil {
			return target, false, fmt.Errorf("停止实例 %s 失败: %w", name, err)
		}
	}
	if target.DriverPID > 0 && ProcessAlive(target.DriverPID) {
		if err := Kill(target.DriverPID); err != nil {
			r.warn("停止驱动进程失败 (PID %d): %v", target.DriverPID, err)
		}
	}

	if err := r.saveLocked(kept); err != nil {
		return target, alreadyExited, err
	}
	return target, alreadyExited, nil
}

// Kill terminates a process by PID, escalating to a force kill after a grace
// period on platforms that support graceful termination.
func Kill(pid int) error {
	if pid <= 0 {
		return fmt.Errorf("无效的进程 ID: %d", pid)
	}
	return killProcess(pid)
}