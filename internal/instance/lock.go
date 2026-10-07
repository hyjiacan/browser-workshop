package instance

import (
	"fmt"
	"os"
	"time"
)

// lockStaleAfter is how old a lock file may be before it is considered
// abandoned and removed. It bounds the damage from a crashed writer.
const lockStaleAfter = 30 * time.Second

// withFileLock runs fn while holding an exclusive lock file. The lock is
// advisory and best-effort: it is created with O_CREATE|O_EXCL and retried with
// backoff, so concurrent bws processes cannot interleave registry writes.
func withFileLock(lockPath string, fn func() error) error {
	deadline := time.Now().Add(5 * time.Second)
	for {
		f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err == nil {
			_ = f.Close()
			defer os.Remove(lockPath)
			return fn()
		}
		if !os.IsExist(err) {
			return fmt.Errorf("获取实例注册表锁失败: %w", err)
		}

		// Break a stale lock left behind by a crashed process.
		if info, statErr := os.Stat(lockPath); statErr == nil {
			if time.Since(info.ModTime()) > lockStaleAfter {
				_ = os.Remove(lockPath)
				continue
			}
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("等待实例注册表锁超时: %s", lockPath)
		}
		time.Sleep(25 * time.Millisecond)
	}
}
