package nativeacl

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const fixture = `<QDocRoot><func><ownContent><owner>NAS_FA</owner>
<richacl_converting>0</richacl_converting><ntfs><list>
<node><principal>user</principal><name>fa_fixture</name>
<ace_rw>3</ace_rw><ace_ro>0</ace_ro><ace_no>1</ace_no><ace_win>0</ace_win></node>
<enable_inherited>0</enable_inherited><inherited_mode>1</inherited_mode>
</list></ntfs></ownContent></func></QDocRoot>`

func TestParse(t *testing.T) {
	s, err := Parse([]byte(fixture))
	if err != nil || s.Complete || s.Total != nil || len(s.Entries) != 1 {
		t.Fatalf("missing count must remain incomplete: %+v %v", s, err)
	}
	if s.Entries[0].ReadWrite != "3" || s.Entries[0].Deny != "1" {
		t.Fatal("mixed inherited RW and explicit deny was lost")
	}
	for _, tc := range []struct {
		name, xml       string
		valid, complete bool
	}{
		{"complete", strings.Replace(fixture, "<list>", "<list><count>1</count>", 1), true, true},
		{"partial", strings.Replace(fixture, "<list>", "<list><count>2</count>", 1), true, false},
		{"bad count", strings.Replace(fixture, "<list>", "<list><count>0</count>", 1), false, false},
		{"login error", "<QDocRoot><authPassed>0</authPassed></QDocRoot>", false, false},
		{"unknown enum", strings.Replace(fixture, "<ace_rw>3", "<ace_rw>9", 1), false, false},
		{"malformed", fixture[:len(fixture)-10], false, false},
		{"converting", strings.Replace(fixture, "<richacl_converting>0", "<richacl_converting>1", 1), true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, err := Parse([]byte(tc.xml))
			if (err == nil) != tc.valid || s.Complete != tc.complete {
				t.Fatalf("got %+v %v", s, err)
			}
			if tc.name == "converting" && !s.Converting {
				t.Fatal("conversion state lost")
			}
		})
	}
}

func TestRead(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != "POST" || r.URL.Path != "/cgi-bin/priv/AccessControl.cgi" || r.URL.RawQuery != "" {
			t.Error("unexpected request")
		}
		if err := r.ParseForm(); err != nil {
			t.Error(err)
		}
		if r.PostForm.Get("func") != "get_ntfs_basic" || r.PostForm.Get("sid") != "test-session" ||
			r.PostForm.Get("target") != "fixture/child" {
			t.Error("invalid read contract")
		}
		for key := range r.PostForm {
			if strings.HasPrefix(key, "action") {
				t.Error("write action in read request")
			}
		}
		_, _ = w.Write([]byte(fixture))
	}))
	defer server.Close()
	s, err := Read(context.Background(), server.URL, "test-session", "fixture/child", []string{"fixture/child"})
	if err != nil || s.Target != "fixture/child" || calls != 1 {
		t.Fatalf("%+v %v calls=%d", s, err, calls)
	}
	_, err = Read(context.Background(), server.URL, "test-session", "fixture/child/other", []string{"fixture/child"})
	if err == nil || calls != 1 {
		t.Fatal("allowlist did not stop request")
	}
}

func TestReadStops(t *testing.T) {
	for _, status := range []int{302, 401, 500} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				w.Header().Set("Location", "/other")
				w.WriteHeader(status)
			}))
			defer server.Close()
			_, err := Read(context.Background(), server.URL, "secret", "fixture", []string{"fixture"})
			if err == nil || calls != 1 || strings.Contains(err.Error(), "secret") {
				t.Fatalf("redirect/retry/secret leak: %v calls=%d", err, calls)
			}
		})
	}
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	_, err := Read(ctx, "http://127.0.0.1:1", "secret", "fixture", []string{"fixture"})
	if err == nil || strings.Contains(err.Error(), "secret") {
		t.Fatalf("deadline not enforced: %v", err)
	}
}

func TestDetailedInheritance(t *testing.T) {
	explicit := `<node><principal>user</principal><name>fa_fixture</name>
<type>deny</type><ace>11111111111111</ace><apply_onto>1</apply_onto>
<container_only>0</container_only><inherited>0</inherited><ancestor>none</ancestor></node>`
	inherited := strings.ReplaceAll(strings.ReplaceAll(explicit,
		"<inherited>0", "<inherited>1"), "<ancestor>none", `<ancestor>\\NAS\share\parent`)
	unknownParent := strings.ReplaceAll(inherited, `\\NAS\share\parent`, "Parent object")
	xml := fixture[:strings.Index(fixture, "<node>")] + explicit + inherited + unknownParent +
		fixture[strings.Index(fixture, "<enable_inherited>"):]
	s, err := ParseDetailed([]byte(xml))
	if err != nil || len(s.Entries) != 3 || s.Complete {
		t.Fatalf("detailed entries collapsed or marked complete: %+v %v", s, err)
	}
	if s.Entries[0].Inherited != "0" || s.Entries[1].Ancestor != `\\NAS\share\parent` ||
		s.Entries[2].Ancestor != "Parent object" {
		t.Fatal("inheritance source lost or fabricated")
	}
	for _, invalid := range []string{
		strings.Replace(xml, "<inherited>0", "<inherited>9", 1),
		strings.Replace(xml, "<ancestor>none</ancestor>", "", 1),
		strings.Replace(xml, "11111111111111", "111", 1),
		strings.Replace(xml, "<type>deny", "<type>unknown", 1),
	} {
		if _, err := ParseDetailed([]byte(invalid)); err == nil {
			t.Fatal("invalid detailed entry accepted")
		}
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Error(err)
		}
		if r.PostForm.Get("func") != "get_ntfs_win_special" {
			t.Error("wrong native view")
		}
		_, _ = w.Write([]byte(xml))
	}))
	defer server.Close()
	s, err = ReadDetailed(context.Background(), server.URL, "session", "fixture", []string{"fixture"})
	if err != nil || len(s.Entries) != 3 {
		t.Fatalf("detailed read failed: %+v %v", s, err)
	}
}
