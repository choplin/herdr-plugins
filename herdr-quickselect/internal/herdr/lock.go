package herdr

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

// AlreadyActiveError reports an existing picker in the same exclusion scope.
type AlreadyActiveError struct{ Scope string }

func (err *AlreadyActiveError) Error() string {
	return "Quick Select is already active in " + err.Scope
}

// SessionLock holds a process-scoped picker lock.
type SessionLock struct{ file *os.File }

// AcquireSessionLock prevents overlapping pickers in one exclusion scope.
func AcquireSessionLock(directory, scope string) (*SessionLock, error) {
	if directory == "" {
		directory = os.TempDir()
	}
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return nil, &SessionError{Operation: "create picker lock directory", Cause: err}
	}
	digest := sha256.Sum256([]byte(scope))
	path := filepath.Join(directory, "active-"+hex.EncodeToString(digest[:12])+".lock")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, &SessionError{Operation: "open picker lock", Cause: err}
	}
	if err := unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		_ = file.Close()
		if errors.Is(err, unix.EWOULDBLOCK) {
			return nil, &AlreadyActiveError{Scope: scope}
		}
		return nil, &SessionError{Operation: "lock picker", Cause: err}
	}
	return &SessionLock{file: file}, nil
}

// Release unlocks the source pane.
func (lock *SessionLock) Release() error {
	if lock == nil || lock.file == nil {
		return nil
	}
	err := errors.Join(unix.Flock(int(lock.file.Fd()), unix.LOCK_UN), lock.file.Close())
	lock.file = nil
	if err != nil {
		return &SessionError{Operation: "unlock picker", Cause: err}
	}
	return nil
}
