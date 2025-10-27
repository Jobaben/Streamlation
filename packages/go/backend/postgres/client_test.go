package postgres

import (
	"strings"
	"testing"
)

func TestPrepareQuery(t *testing.T) {
	tests := map[string]struct {
		query   string
		args    []any
		want    string
		wantErr string
	}{
		"simple substitution": {
			query: "SELECT $1",
			args:  []any{"value"},
			want:  "SELECT 'value'",
		},
		"single quoted literal is preserved": {
			query: "SELECT '$1', $1",
			args:  []any{"value"},
			want:  "SELECT '$1', 'value'",
		},
		"dollar quoted literal is preserved": {
			query: "SELECT $$literal $2$$, $1",
			args:  []any{"value"},
			want:  "SELECT $$literal $2$$, 'value'",
		},
		"missing parameter": {
			query:   "SELECT $2",
			args:    []any{"value"},
			wantErr: "missing parameter",
		},
	}

	for name, tt := range tests {
		tt := tt
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got, err := prepareQuery(tt.query, tt.args...)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error containing %q, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("unexpected query: got %q, want %q", got, tt.want)
			}
		})
	}
}
