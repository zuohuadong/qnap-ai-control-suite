package jobs

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	qexec "qnap-ai-control-suite/agent/internal/exec"
)

func TestPersistentLogsAndResultSurviveRestart(t *testing.T) {
	dir := t.TempDir()
	journal := filepath.Join(dir, "jobs.jsonl")
	manager := NewWithOptions(Options{MaxHistory: 10, JournalPath: journal})
	job := manager.Start("durable", func(_ context.Context, log func(string)) (any, error) {
		log("first")
		log("second")
		return map[string]any{"state": "ready", "token": "must-not-persist"}, nil
	})
	waitForStatus(t, manager, job.ID, Succeeded)

	restarted := NewWithOptions(Options{MaxHistory: 10, JournalPath: journal})
	current, ok := restarted.Get(job.ID)
	if !ok || current.Status != Succeeded || current.Result == nil {
		t.Fatalf("durable result not restored: %#v exists=%v", current, ok)
	}
	lines, next, truncated, ok := restarted.Logs(job.ID, 0, 10)
	if !ok || truncated || next != 2 || len(lines) != 2 || lines[0] != "first" || lines[1] != "second" {
		t.Fatalf("durable logs=%#v next=%d truncated=%v ok=%v", lines, next, truncated, ok)
	}
	data, err := os.ReadFile(journal)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) == "" {
		t.Fatal("journal unexpectedly empty")
	}
	if bytesContain(data, []byte("must-not-persist")) {
		t.Fatal("secret-like result value was persisted")
	}
}

func TestInterruptedRecoveryRequiresInspectionAndNeverAutoRetries(t *testing.T) {
	dir := t.TempDir()
	journal := filepath.Join(dir, "jobs.jsonl")
	manager := NewWithOptions(Options{MaxHistory: 10, JournalPath: journal})
	release := make(chan struct{})
	job := manager.Start("long", func(context.Context, func(string)) (any, error) {
		<-release
		return nil, nil
	})
	waitForStatus(t, manager, job.ID, Running)

	restarted := NewWithOptions(Options{MaxHistory: 10, JournalPath: journal})
	current, ok := restarted.Get(job.ID)
	if !ok || current.Status != Interrupted || !current.Recovered || current.RecoveryStatus != "needs_inspection" || current.Retriable {
		t.Fatalf("unexpected recovered job: %#v exists=%v", current, ok)
	}
	close(release)
	waitForStatus(t, manager, job.ID, Succeeded)
}

func TestJournalCompactionSnapshotCanRestoreState(t *testing.T) {
	dir := t.TempDir()
	journal := filepath.Join(dir, "jobs.jsonl")
	snapshot := filepath.Join(dir, "jobs.snapshot.json")
	manager := NewWithOptions(Options{MaxHistory: 10, JournalPath: journal, SnapshotPath: snapshot, CompactBytes: 1})
	job := manager.Start("compact", func(_ context.Context, log func(string)) (any, error) {
		log("persisted")
		return map[string]any{"state": "done"}, nil
	})
	waitForStatus(t, manager, job.ID, Succeeded)
	if _, err := os.Stat(snapshot); err != nil {
		t.Fatalf("snapshot was not created: %v", err)
	}

	restarted := NewWithOptions(Options{MaxHistory: 10, JournalPath: journal, SnapshotPath: snapshot, CompactBytes: 1})
	current, ok := restarted.Get(job.ID)
	if !ok || current.Status != Succeeded {
		t.Fatalf("snapshot did not restore job: %#v exists=%v", current, ok)
	}
	lines, _, _, ok := restarted.Logs(job.ID, 0, 10)
	if !ok || len(lines) != 1 || lines[0] != "persisted" {
		t.Fatalf("snapshot restore lost log: %#v", lines)
	}
}

func TestRedactedCommandOutputNeverReachesJobMetadataOrLogFile(t *testing.T) {
	dir := t.TempDir()
	journal := filepath.Join(dir, "jobs.jsonl")
	manager := NewWithOptions(Options{MaxHistory: 10, JournalPath: journal, RedactSecrets: true})
	job := manager.Start("redacted", func(_ context.Context, log func(string)) (any, error) {
		log("qacs-test-secret-123 qacs-test-password-123")
		return qexec.Result{Argv: []string{"/bin/echo", "qacs-test-secret-123"}, Stdout: "qacs-test-secret-123", Stderr: "qacs-test-password-123"}, nil
	})
	waitForStatus(t, manager, job.ID, Succeeded)
	current, ok := manager.Get(job.ID)
	if !ok {
		t.Fatal("redacted job was not retained")
	}
	encoded := mustJSON(t, current.Result)
	if strings.Contains(encoded, "qacs-test-secret-123") || strings.Contains(encoded, "qacs-test-password-123") {
		t.Fatalf("redacted command result leaked: %s", encoded)
	}
	lines, _, _, ok := manager.Logs(job.ID, 0, 10)
	if !ok || len(lines) != 1 || strings.Contains(strings.Join(lines, ""), "qacs-test-secret-123") || strings.Contains(strings.Join(lines, ""), "qacs-test-password-123") {
		t.Fatalf("redacted job logs leaked: %#v", lines)
	}
	data, err := os.ReadFile(filepath.Join(dir, "jobs.jsonl.logs", job.ID+".jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "qacs-test-secret-123") || strings.Contains(string(data), "qacs-test-password-123") {
		t.Fatalf("persistent job log leaked: %s", data)
	}
}

func mustJSON(t *testing.T, value any) string {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func bytesContain(haystack, needle []byte) bool {
	if len(needle) == 0 {
		return true
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		match := true
		for j := range needle {
			if haystack[i+j] != needle[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
