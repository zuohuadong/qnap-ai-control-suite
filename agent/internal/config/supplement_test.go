package config

import "testing"

func TestSupplementConfig(t *testing.T) {
	valid := func() Config {
		cfg := Defaults()
		cfg.Auth.TokenSHA256 = "test-hash"
		cfg.SupplementReadOnly = true
		cfg.Permissions.AllowedRoots = []string{"/share/fa-acceptance"}
		return cfg
	}
	if _, err := Normalize(valid()); err != nil {
		t.Fatal(err)
	}
	cases := map[string]func(*Config){
		"full_trust":      func(c *Config) { c.Profile = "full_trust" },
		"shell":           func(c *Config) { c.Permissions.AllowShell = true },
		"commands":        func(c *Config) { c.Permissions.AllowedCommands = []string{"/bin/sh"} },
		"any_command":     func(c *Config) { c.Permissions.AllowAnyCommand = true },
		"root":            func(c *Config) { c.Permissions.AllowedRoots = []string{"/"} },
		"audit":           func(c *Config) { c.Audit.Enabled = false },
		"privacy":         func(c *Config) { c.Privacy.RedactSecrets = false },
		"audit_redaction": func(c *Config) { c.Audit.RedactSecrets = boolPtr(false) },
		"adapter":         func(c *Config) { c.QNAPAdapters["shares"] = QNAPAdapter{} },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			cfg := valid()
			mutate(&cfg)
			if _, err := Normalize(cfg); err == nil {
				t.Fatal("unsafe configuration accepted")
			}
		})
	}
	cfg, err := Load("../../../configs/smb-supplement-readonly.json")
	if err != nil || !cfg.SupplementReadOnly {
		t.Fatalf("example configuration: %v", err)
	}
}
