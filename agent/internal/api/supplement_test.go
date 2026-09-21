package api

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSupplementReadOnly(t *testing.T) {
	s, token := testServer(t)
	// 即使误配 full_trust，补充模式也必须阻止写入和任意命令。
	s.Config.SupplementReadOnly = true
	for _, path := range []string{
		"/v1/exec", "/v1/shell", "/v1/command/run", "/v1/jobs",
		"/v1/acl/set", "/v1/files/write", "/v1/files/manage",
		"/v1/files/tree", "/v1/storage/snapshots/action",
		"/v1/shares/manage", "/v1/qnap/probe", "/v1/unknown",
	} {
		for _, method := range []string{"GET", "POST", "PUT", "DELETE"} {
			w := request(t, s, token, method, path, `{}`)
			if w.Code != http.StatusForbidden || !strings.Contains(w.Body.String(), "supplement_read_only") {
				t.Fatalf("%s %s: %d %s", method, path, w.Code, w.Body)
			}
		}
	}
	if w := request(t, s, "", "GET", "/v1/health", ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("authentication bypass: %d", w.Code)
	}
	if w := request(t, s, token, "GET", "/v1/health", ""); w.Code != http.StatusOK {
		t.Fatalf("health: %d %s", w.Code, w.Body)
	}
	root := t.TempDir()
	s.Files.Roots = []string{root}
	path := filepath.Join(root, "fixture.txt")
	if err := os.WriteFile(path, []byte("supplement fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	w := request(t, s, token, "POST", "/v1/files/checksum", `{"path":"`+path+`"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("checksum: %d %s", w.Code, w.Body)
	}
	outside := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(outside, []byte("outside"), 0600); err != nil {
		t.Fatal(err)
	}
	w = request(t, s, token, "POST", "/v1/files/read", `{"path":"`+outside+`"}`)
	if w.Code == http.StatusOK {
		t.Fatal("read escaped allowed roots")
	}
}

func TestSupplementReadAllowed(t *testing.T) {
	for _, path := range []string{"/v1/health", "/v1/shares", "/v1/shares/smb-status", "/v1/files/list", "/v1/files/stat"} {
		if !supplementReadAllowed("GET", path) || supplementReadAllowed("POST", path) {
			t.Fatalf("invalid method policy for %s", path)
		}
	}
	for _, path := range []string{"/v1/files/read", "/v1/files/checksum"} {
		if !supplementReadAllowed("POST", path) || supplementReadAllowed("GET", path) {
			t.Fatalf("invalid method policy for %s", path)
		}
	}
}
