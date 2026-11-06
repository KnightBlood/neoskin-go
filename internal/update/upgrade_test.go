package update

import "testing"

func TestParseVersionNormalizesPatchSuffix(t *testing.T) {
	cases := map[string][]int{
		"6.0.11_patch": {6, 0, 11},
		"v5.0.0":       {5, 0, 0},
		"6":            {6},
		"":             nil,
		"not-version":  nil,
	}
	for input, want := range cases {
		got := ParseVersion(input)
		if len(got) != len(want) {
			t.Fatalf("ParseVersion(%q) = %v, want %v", input, got, want)
		}
		for index := range want {
			if got[index] != want[index] {
				t.Fatalf("ParseVersion(%q) = %v, want %v", input, got, want)
			}
		}
	}
}

func TestIsNewer(t *testing.T) {
	if !IsNewer("6.0.12", "6.0.11_patch") {
		t.Fatal("6.0.12 should be newer than 6.0.11_patch")
	}
	if IsNewer("6.0.11_patch", "6.0.11") {
		t.Fatal("_patch should compare equal to the stable version")
	}
	if IsNewer("5.0.0", "6.0.11_patch") {
		t.Fatal("5.0.0 is older")
	}
	if !IsNewer("5.0.0", "4.9.9") {
		t.Fatal("5.0.0 should be newer than 4.9.9")
	}
}

func TestNeedsUpdateTreatsMissingVersionAsInstalled(t *testing.T) {
	if NeedsUpdate("") {
		t.Fatal("missing options.version must not trigger the PHP update middleware")
	}
}

func TestProceduresUseLessThanSemantics(t *testing.T) {
	// 对应 PHP: Comparator::lessThan(option('version'), $version)
	if !IsNewer("5.0.0", "") {
		t.Fatal("empty stored version should run the 5.0.0 procedure")
	}
	if IsNewer("5.0.0", "5.0.0") {
		t.Fatal("equal version must not run the procedure")
	}
	if IsNewer("0.0.1", "4.9.9") {
		t.Fatal("4.9.9 is newer than the 0.0.1 procedure")
	}
}
