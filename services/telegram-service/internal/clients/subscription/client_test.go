package subscription

import "testing"

func TestNormalizeAddr(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "empty", in: "  ", want: ""},
		{name: "port only", in: ":9092", want: "localhost:9092"},
		{name: "host port", in: " subscription-service:9092 ", want: "subscription-service:9092"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeAddr(tt.in); got != tt.want {
				t.Fatalf("normalizeAddr(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestClientValidatesAPI(t *testing.T) {
	client := &Client{}

	if _, err := client.StartTrial(nil, "user-1"); err == nil {
		t.Fatal("StartTrial expected error")
	}
	if _, err := client.GetActive(nil, "user-1"); err == nil {
		t.Fatal("GetActive expected error")
	}
	if _, err := client.ActivateOrRenew(nil, ActivateOrRenewInput{}); err == nil {
		t.Fatal("ActivateOrRenew expected error")
	}
}
