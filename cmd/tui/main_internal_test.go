package main

import "testing"

func TestOpenStore(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{"json extension", "registry.json", false},
		{"unsupported extension", "registry.txt", true},
		{"no extension", "registry", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s, err := openStore(tc.path)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if s == nil {
				t.Fatal("expected non-nil Store")
			}
		})
	}
}
