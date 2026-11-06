package check

import (
	"errors"
	"strings"
	"testing"
)

func TestCoveragePercent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		profile    string
		want       float64
		noStmts    bool
		wantAnyErr bool
	}{
		{name: "all statements covered", profile: "mode: atomic\nexample.go:1.1,2.2 2 1\n", want: 100},
		{name: "some statements covered", profile: "mode: atomic\nexample.go:1.1,2.2 2 1\nexample.go:3.1,4.2 2 0\n", want: 50},
		{name: "mixed partial counts", profile: "mode: set\nexample.go:1.1,2.2 1 3\nexample.go:3.1,4.2 3 0\n", want: 25},
		{name: "mode line only", profile: "mode: atomic\n", noStmts: true},
		{name: "empty profile", profile: "", noStmts: true},
		{name: "malformed line", profile: "mode: atomic\nexample.go:1.1,2.2\n", wantAnyErr: true},
		{name: "non numeric statements", profile: "mode: atomic\nexample.go:1.1,2.2 two 1\n", wantAnyErr: true},
		{name: "negative count", profile: "mode: atomic\nexample.go:1.1,2.2 2 -1\n", wantAnyErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := CoveragePercent(strings.NewReader(tt.profile))
			switch {
			case tt.noStmts:
				if !errors.Is(err, ErrNoStatements) {
					t.Fatalf("error = %v, want ErrNoStatements", err)
				}
			case tt.wantAnyErr:
				if err == nil {
					t.Fatalf("percent = %v, want error", got)
				}
			case err != nil:
				t.Fatalf("error = %v", err)
			case got != tt.want:
				t.Fatalf("percent = %v, want %v", got, tt.want)
			}
		})
	}
}
