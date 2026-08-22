package theme

import (
	"testing"
)

func TestThemeEngine(t *testing.T) {
	te := NewThemeEngine("kinetic")
	if te.Active().Name != "kinetic" {
		t.Fatalf("Expected active theme 'kinetic', got '%s'", te.Active().Name)
	}

	themes := te.ListThemes()
	if len(themes) < 5 {
		t.Fatalf("Expected at least 5 themes, got %d", len(themes))
	}

	// Switch theme
	ok := te.SetTheme("dracula")
	if !ok || te.Active().Name != "dracula" {
		t.Fatalf("Failed to switch to dracula theme")
	}

	ok = te.SetTheme("matrix")
	if !ok || te.Active().Name != "matrix" {
		t.Fatalf("Failed to switch to matrix theme")
	}

	p := te.P("test")
	if p == "" || p == "test" {
		t.Fatalf("Expected ANSI colored text")
	}
}
