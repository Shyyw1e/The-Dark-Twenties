package subscription

import "testing"

func TestNormalizeAddr(t *testing.T) {
	tests := []struct {
		name string
		addr string
		want string
	}{
		{name: "host port", addr: "subscription-service:9092", want: "subscription-service:9092"},
		{name: "local port", addr: ":9092", want: "localhost:9092"},
		{name: "spaces", addr: "  localhost:9092  ", want: "localhost:9092"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeAddr(tt.addr); got != tt.want {
				t.Fatalf("normalizeAddr() = %q, want %q", got, tt.want)
			}
		})
	}
}
