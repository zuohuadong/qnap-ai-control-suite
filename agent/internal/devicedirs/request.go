package devicedirs

import (
	"errors"
	"regexp"
	"strings"
	"unicode"
)

type Request struct {
	Tenant        string `json:"tenant"`
	Device        string `json:"device"`
	Stage         string `json:"stage"`
	CaseDirectory string `json:"caseDirectory"`
}

type Receipt struct {
	Request
	Verified bool `json:"verified"`
}

var tenantPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,80}$`)
var devicePattern = regexp.MustCompile(`^.+-[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func (r Request) Validate() error {
	if !tenantPattern.MatchString(r.Tenant) || !devicePattern.MatchString(r.Device) {
		return errors.New("invalid device identity")
	}
	for _, part := range []string{r.Device, r.CaseDirectory} {
		if part == "" || part == "." || part == ".." || len(part) > 255 ||
			strings.ContainsAny(part, `/\`) || strings.IndexFunc(part, unicode.IsControl) >= 0 {
			return errors.New("invalid directory component")
		}
	}
	switch r.Stage {
	case "01_待上传", "02_处理中", "03_已归集", "04_历史记录", "99_异常待确认":
	default:
		return errors.New("invalid stage")
	}
	return nil
}
