package cmd

import (
	"strings"
	"testing"
)

func TestParsePortSpec(t *testing.T) {
	tests := []struct {
		name              string
		spec              string
		wantLocalPort     int
		wantContainerPort int
		wantErr           bool
	}{
		{
			name:              "valid pair",
			spec:              "8080:80",
			wantLocalPort:     8080,
			wantContainerPort: 80,
		},
		{
			name:              "single port",
			spec:              "3000",
			wantLocalPort:     3000,
			wantContainerPort: 3000,
		},
		{
			name:    "invalid local port non-numeric",
			spec:    "abc:80",
			wantErr: true,
		},
		{
			name:    "invalid container port non-numeric",
			spec:    "8080:xyz",
			wantErr: true,
		},
		{
			name:    "missing container port",
			spec:    "8080:",
			wantErr: true,
		},
		{
			name:    "missing local port",
			spec:    ":80",
			wantErr: true,
		},
		{
			name:    "too many colons",
			spec:    "80:80:80",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lp, cp, err := parsePortSpec(tt.spec)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parsePortSpec(%q) error = %v, wantErr %v", tt.spec, err, tt.wantErr)
			}
			if !tt.wantErr {
				if lp != tt.wantLocalPort || cp != tt.wantContainerPort {
					t.Errorf("parsePortSpec(%q) = (%d, %d), want (%d, %d)", tt.spec, lp, cp, tt.wantLocalPort, tt.wantContainerPort)
				}
			}
		})
	}
}

func TestParsePortForwardArgs(t *testing.T) {
	t.Run("two args service and port pair", func(t *testing.T) {
		svc, task, lp, cp, err := parsePortForwardArgs([]string{"api", "8080:80"}, "", 0, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if svc != "api" || task != "" || lp != 8080 || cp != 80 {
			t.Errorf("got (%q, %q, %d, %d), want ('api', '', 8080, 80)", svc, task, lp, cp)
		}
	})

	t.Run("two args service and single port", func(t *testing.T) {
		svc, task, lp, cp, err := parsePortForwardArgs([]string{"web", "443"}, "", 0, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if svc != "web" || task != "" || lp != 443 || cp != 443 {
			t.Errorf("got (%q, %q, %d, %d), want ('web', '', 443, 443)", svc, task, lp, cp)
		}
	})

	t.Run("one arg port spec with task flag", func(t *testing.T) {
		svc, task, lp, cp, err := parsePortForwardArgs([]string{"8080:80"}, "task-abc", 0, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if svc != "" || task != "task-abc" || lp != 8080 || cp != 80 {
			t.Errorf("got (%q, %q, %d, %d), want ('', 'task-abc', 8080, 80)", svc, task, lp, cp)
		}
	})

	t.Run("one arg service with port flags", func(t *testing.T) {
		svc, task, lp, cp, err := parsePortForwardArgs([]string{"api"}, "", 9000, 9000)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if svc != "api" || task != "" || lp != 9000 || cp != 9000 {
			t.Errorf("got (%q, %q, %d, %d), want ('api', '', 9000, 9000)", svc, task, lp, cp)
		}
	})

	t.Run("zero args with task and port flags", func(t *testing.T) {
		svc, task, lp, cp, err := parsePortForwardArgs([]string{}, "task-xyz", 5432, 5432)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if svc != "" || task != "task-xyz" || lp != 5432 || cp != 5432 {
			t.Errorf("got (%q, %q, %d, %d), want ('', 'task-xyz', 5432, 5432)", svc, task, lp, cp)
		}
	})

	t.Run("missing target error", func(t *testing.T) {
		_, _, _, _, err := parsePortForwardArgs([]string{"8080:80"}, "", 0, 0)
		if err == nil || !strings.Contains(err.Error(), "must specify a service name or --task") {
			t.Fatalf("expected missing target error, got %v", err)
		}
	})

	t.Run("port out of range", func(t *testing.T) {
		_, _, _, _, err := parsePortForwardArgs([]string{"api", "70000:80"}, "", 0, 0)
		if err == nil || !strings.Contains(err.Error(), "invalid local port") {
			t.Fatalf("expected port out of range error, got %v", err)
		}
	})

	t.Run("too many args", func(t *testing.T) {
		_, _, _, _, err := parsePortForwardArgs([]string{"api", "8080:80", "extra"}, "", 0, 0)
		if err == nil || !strings.Contains(err.Error(), "too many arguments") {
			t.Fatalf("expected too many arguments error, got %v", err)
		}
	})
}
