package user

import "testing"

func TestNormalizeAddr(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "empty", in: "  ", want: ""},
		{name: "port only", in: ":9091", want: "localhost:9091"},
		{name: "host port", in: " user-service:9091 ", want: "user-service:9091"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeAddr(tt.in); got != tt.want {
				t.Fatalf("normalizeAddr(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
