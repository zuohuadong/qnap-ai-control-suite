//go:build linux

package devicedirs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsure(t *testing.T) {
	root := t.TempDir()
	r := Request{Tenant: "xgic", Device: "test-3212d2fa-0e91-4934-be2c-7c44d7ced872", Stage: "01_待上传", CaseDirectory: "FA-26-01013_sample"}
	device := filepath.Join(root, r.Tenant, r.Device)
	if _, err := Ensure(root, r); err == nil {
		t.Fatal("created missing device")
	}
	if err := os.MkdirAll(device, 0700); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if got, err := Ensure(root, r); err != nil || !got.Verified {
			t.Fatalf("%+v %v", got, err)
		}
	}
	file := filepath.Join(device, r.Stage, r.CaseDirectory, "evidence")
	if err := os.WriteFile(file, []byte("unchanged"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Ensure(root, r); err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(file); err != nil || string(b) != "unchanged" {
		t.Fatal("changed data")
	}
	for _, bad := range []string{"../escape", "/absolute", "a/b", "a\\b", "bad\x00name", "."} {
		invalid := r
		invalid.CaseDirectory = bad
		if _, err := Ensure(root, invalid); err == nil {
			t.Fatal("accepted invalid component")
		}
	}
	r.Stage = "02_处理中"
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(device, r.Stage)); err != nil {
		t.Fatal(err)
	}
	if _, err := Ensure(root, r); err == nil {
		t.Fatal("followed symlink")
	}
	entries, err := os.ReadDir(outside)
	if err != nil || len(entries) != 0 {
		t.Fatal("escaped root")
	}
	r.Stage = "03_已归集"
	if err := os.WriteFile(filepath.Join(device, r.Stage), nil, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Ensure(root, r); err == nil {
		t.Fatal("replaced file")
	}
}
