package middleware

import "testing"

func TestIsSetupPathUsesSegmentMatching(t *testing.T) {
	for _, path := range []string{"/setup", "setup", "/setup/database", "setup/info"} {
		if !isSetupPath(path) {
			t.Fatalf("%q should be treated as setup", path)
		}
	}
	for _, path := range []string{"/setupx", "/", "/auth/login", "/setup-foo"} {
		if isSetupPath(path) {
			t.Fatalf("%q should not be treated as setup", path)
		}
	}
}
