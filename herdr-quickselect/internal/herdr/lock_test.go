package herdr

import (
	"errors"
	"testing"
)

func TestSessionLockRejectsSamePaneUntilReleased(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	first, err := AcquireSessionLock(directory, "workspace:w1")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = first.Release() })

	_, err = AcquireSessionLock(directory, "workspace:w1")
	var active *AlreadyActiveError
	if !errors.As(err, &active) {
		t.Fatalf("AcquireSessionLock() error = %v, want AlreadyActiveError", err)
	}
	if err := first.Release(); err != nil {
		t.Fatal(err)
	}
	second, err := AcquireSessionLock(directory, "workspace:w1")
	if err != nil {
		t.Fatalf("AcquireSessionLock() after release error = %v", err)
	}
	_ = second.Release()
}

func TestSessionLockAllowsDifferentWorkspaces(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	first, err := AcquireSessionLock(directory, "workspace:w1")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = first.Release() }()
	second, err := AcquireSessionLock(directory, "workspace:w2")
	if err != nil {
		t.Fatal(err)
	}
	_ = second.Release()
}
