package doctor

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDoctorDiagnostics(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "doctor_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cfgPath := filepath.Join(tempDir, "config.yaml")

	report := RunDiagnostics(cfgPath, true)
	if report == nil {
		t.Fatal("Expected non-nil doctor report")
	}

	if report.Passed == 0 {
		t.Fatal("Expected at least one passed check in doctor diagnostics")
	}
}
