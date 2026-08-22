package updater

import (
	"testing"
)

func TestUpdaterGetVersion(t *testing.T) {
	v := GetVersion()
	if v == "" {
		t.Fatal("Expected non-empty version string")
	}

	res, err := RunUpdate(".", true)
	if err != nil {
		t.Fatalf("Check update failed: %v", err)
	}
	if res.CurrentVersion != Version {
		t.Fatalf("Expected version %s, got %s", Version, res.CurrentVersion)
	}
}
