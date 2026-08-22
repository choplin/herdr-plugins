//go:build linux || darwin

package identity

import (
	"os"
	"path/filepath"
	"syscall"
)

// LockError reports a failure while managing the reconciliation lock.
type LockError struct {
	Operation string
	Path      string
	Err       error
}

func (err *LockError) Error() string {
	return err.Operation + " " + err.Path + ": " + err.Err.Error()
}

// Unwrap exposes the underlying filesystem or locking error.
func (err *LockError) Unwrap() error {
	return err.Err
}

// WithReconcileLock serializes concurrent startup and event reconciliations.
func WithReconcileLock(stateDir string, reconcile func() (int, error)) (int, error) {
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return 0, &LockError{Operation: "create state directory", Path: stateDir, Err: err}
	}

	lockPath := filepath.Join(stateDir, "reconcile.lock")
	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return 0, &LockError{Operation: "open lock file", Path: lockPath, Err: err}
	}
	defer func() {
		_ = lockFile.Close()
	}()

	if err := syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX); err != nil {
		return 0, &LockError{Operation: "lock", Path: lockPath, Err: err}
	}
	failures, reconcileErr := reconcile()
	unlockErr := syscall.Flock(int(lockFile.Fd()), syscall.LOCK_UN)
	if reconcileErr != nil {
		return failures, reconcileErr
	}
	if unlockErr != nil {
		return failures, &LockError{Operation: "unlock", Path: lockPath, Err: unlockErr}
	}
	return failures, nil
}
