package main

import (
	"context"
	"errors"
	"testing"

	"github.com/choplin/herdr-quickselect/internal/config"
)

type recordingNotifier struct {
	titles []string
	err    error
}

func (notifier *recordingNotifier) ShowNotification(_ context.Context, title string) error {
	notifier.titles = append(notifier.titles, title)
	return notifier.err
}

func TestNotifyActionSuccessReportsClipboardCopy(t *testing.T) {
	t.Parallel()

	notifier := &recordingNotifier{}
	notifyActionSuccess(
		context.Background(), config.Action{Type: "clipboard"}, nil, notifier,
	)

	if len(notifier.titles) != 1 || notifier.titles[0] != "Copied to clipboard" {
		t.Fatalf("notification titles = %#v, want [Copied to clipboard]", notifier.titles)
	}
}

func TestNotifyActionSuccessReportsOpenedURL(t *testing.T) {
	t.Parallel()

	notifier := &recordingNotifier{}
	notifyActionSuccess(
		context.Background(), config.Action{Type: "open"}, nil, notifier,
	)

	if len(notifier.titles) != 1 || notifier.titles[0] != "Opened URL in browser" {
		t.Fatalf("notification titles = %#v, want [Opened URL in browser]", notifier.titles)
	}
}

func TestNotifyActionSuccessSkipsFailedAndUnreportedActions(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name      string
		action    config.Action
		actionErr error
	}{
		{name: "failed clipboard", action: config.Action{Type: "clipboard"}, actionErr: errors.New("copy failed")},
		{name: "failed open", action: config.Action{Type: "open"}, actionErr: errors.New("open failed")},
		{name: "exec", action: config.Action{Type: "exec"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			notifier := &recordingNotifier{}
			notifyActionSuccess(context.Background(), test.action, test.actionErr, notifier)
			if len(notifier.titles) != 0 {
				t.Fatalf("notification titles = %#v, want none", notifier.titles)
			}
		})
	}
}

func TestNotifyActionSuccessIgnoresNotificationFailure(t *testing.T) {
	t.Parallel()

	notifier := &recordingNotifier{err: errors.New("notification unavailable")}
	notifyActionSuccess(
		context.Background(), config.Action{Type: "clipboard"}, nil, notifier,
	)

	if len(notifier.titles) != 1 {
		t.Fatalf("notification calls = %d, want 1", len(notifier.titles))
	}
}
