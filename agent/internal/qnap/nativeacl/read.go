// Package nativeacl reads the NAS system ACL view without implementing ACL rules.
package nativeacl

import (
	"context"
	"encoding/xml"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Entry struct {
	Principal     string `xml:"principal"`
	Name          string `xml:"name"`
	ReadWrite     string `xml:"ace_rw"`
	ReadOnly      string `xml:"ace_ro"`
	Deny          string `xml:"ace_no"`
	Windows       string `xml:"ace_win"`
	Type          string `xml:"type"`
	ACE           string `xml:"ace"`
	ApplyOnto     string `xml:"apply_onto"`
	ContainerOnly string `xml:"container_only"`
	Inherited     string `xml:"inherited"`
	Ancestor      string `xml:"ancestor"`
}

type Snapshot struct {
	Target          string
	Owner           string
	Converting      bool
	EnableInherited string
	InheritedMode   string
	Entries         []Entry
	Total           *int
	Complete        bool
}

type document struct {
	XMLName xml.Name `xml:"QDocRoot"`
	Content struct {
		Owner      string `xml:"owner"`
		Converting string `xml:"richacl_converting"`
		List       struct {
			Count           *int    `xml:"count"`
			Entries         []Entry `xml:"node"`
			EnableInherited string  `xml:"enable_inherited"`
			InheritedMode   string  `xml:"inherited_mode"`
		} `xml:"ntfs>list"`
	} `xml:"func>ownContent"`
}

func Parse(data []byte) (Snapshot, error) {
	return parse(data, false)
}

// ParseDetailed 保留每条 ACE 的来源，不按账号折叠显式项与继承项。
func ParseDetailed(data []byte) (Snapshot, error) {
	return parse(data, true)
}

func parse(data []byte, detailed bool) (Snapshot, error) {
	var doc document
	if len(data) > 2_000_000 || xml.Unmarshal(data, &doc) != nil {
		return Snapshot{}, errors.New("invalid native ACL XML")
	}
	c := doc.Content
	if c.Converting != "0" && c.Converting != "1" {
		return Snapshot{}, errors.New("missing native ACL conversion state")
	}
	if (c.List.EnableInherited != "0" && c.List.EnableInherited != "1") ||
		(c.List.InheritedMode != "0" && c.List.InheritedMode != "1") {
		return Snapshot{}, errors.New("unknown native ACL inheritance state")
	}
	seen := make(map[string]bool)
	for _, e := range c.List.Entries {
		if e.Principal == "" || e.Name == "" {
			return Snapshot{}, errors.New("missing native ACL principal")
		}
		if detailed {
			if (e.Type != "allow" && e.Type != "deny") ||
				(e.Inherited != "0" && e.Inherited != "1") ||
				e.Ancestor == "" || len(e.ACE) != 14 ||
				strings.Trim(e.ACE, "0123") != "" ||
				e.ApplyOnto == "" ||
				(e.ContainerOnly != "0" && e.ContainerOnly != "1") {
				return Snapshot{}, errors.New("invalid native detailed ACL entry")
			}
			// 真机可返回同账号的多条 ACE，甚至重复来源，必须原样保留。
			continue
		}
		key := e.Principal + "\x00" + e.Name
		if seen[key] {
			return Snapshot{}, errors.New("duplicate native ACL principal")
		}
		seen[key] = true
		for _, value := range []string{e.ReadWrite, e.ReadOnly, e.Deny, e.Windows} {
			if value != "0" && value != "1" && value != "2" && value != "3" {
				return Snapshot{}, errors.New("unknown native ACL enum")
			}
		}
	}
	if c.List.Count != nil && (*c.List.Count < len(c.List.Entries) || *c.List.Count > 1024) {
		return Snapshot{}, errors.New("invalid native ACL count")
	}
	return Snapshot{
		Owner: c.Owner, Converting: c.Converting == "1",
		EnableInherited: c.List.EnableInherited, InheritedMode: c.List.InheritedMode,
		Entries: c.List.Entries, Total: c.List.Count,
		// 未提供总数时保留数据，但不声称读取完整，更不能据此删除。
		Complete: c.List.Count != nil && *c.List.Count == len(c.List.Entries),
	}, nil
}

// Read uses an existing in-memory NAS session. It never retries or writes ACLs.
// allowedTargets is an exact allowlist, not a recursive path prefix.
func Read(ctx context.Context, base, sid, target string, allowedTargets []string) (Snapshot, error) {
	return read(ctx, base, sid, target, allowedTargets, false)
}

func ReadDetailed(ctx context.Context, base, sid, target string, allowedTargets []string) (Snapshot, error) {
	return read(ctx, base, sid, target, allowedTargets, true)
}

func read(ctx context.Context, base, sid, target string, allowedTargets []string, detailed bool) (Snapshot, error) {
	allowed := false
	for _, candidate := range allowedTargets {
		allowed = allowed || candidate == target
	}
	if !allowed || target == "" || sid == "" {
		return Snapshot{}, errors.New("native ACL target or session not allowed")
	}
	u, err := url.Parse(base)
	if err != nil || u.Host == "" || u.User != nil ||
		(u.Scheme != "http" && u.Scheme != "https") {
		return Snapshot{}, errors.New("invalid native NAS endpoint")
	}
	u.Path, u.RawPath, u.RawQuery, u.Fragment = "/cgi-bin/priv/AccessControl.cgi", "", "", ""
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	form := url.Values{
		"sid": {sid}, "func": {"get_ntfs_basic"}, "target": {target},
		"getdataex": {"1"}, "filter": {""}, "type": {"0"},
		"lower": {"0"}, "upper": {strconv.Itoa(1024)}, "refresh": {"0"},
	}
	if detailed {
		form.Set("func", "get_ntfs_win_special")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), strings.NewReader(form.Encode()))
	if err != nil {
		return Snapshot{}, errors.New("cannot build native ACL read")
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	client := &http.Client{
		Transport:     &http.Transport{Proxy: nil, DisableKeepAlives: true},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	res, err := client.Do(req)
	if err != nil {
		// 不返回可能携带地址或认证信息的底层请求错误。
		return Snapshot{}, errors.New("native ACL read failed or timed out; no retry")
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return Snapshot{}, errors.New("native ACL read returned non-success HTTP status")
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, 2_000_001))
	if err != nil {
		return Snapshot{}, errors.New("native ACL response incomplete")
	}
	snapshot, err := parse(data, detailed)
	if err != nil {
		return Snapshot{}, err
	}
	snapshot.Target = target
	return snapshot, nil
}
