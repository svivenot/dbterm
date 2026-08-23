package aiview

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"dbterm/internal/ai"
	"dbterm/internal/config"
)

func TestAIModalLifecycle(t *testing.T) {
	cfg := config.AIConfig{Enabled: true}
	mod := New(cfg)

	if mod.Active {
		t.Fatalf("Expected AIModal to be inactive on creation")
	}

	// 1. Open Modal
	mod.Open(nil, "SELECT * FROM MissingTable;", "Invalid object name 'MissingTable'")
	if !mod.Active {
		t.Fatalf("Expected AIModal to be active after Open()")
	}
	if mod.Mode != ai.AIModeFixError {
		t.Errorf("Expected mode to be FixError when error is present, got %v", mod.Mode)
	}

	// 2. Test Switch Mode with Tab
	mod.NeedsDownload = false
	mod, _ = mod.Update(tea.KeyMsg{Type: tea.KeyTab})
	if mod.Mode != ai.AIModeExplain {
		t.Errorf("Expected mode to switch to Explain, got %v", mod.Mode)
	}

	// 3. Test View rendering
	mod.SetSize(100, 40)
	view := mod.View()
	if !strings.Contains(view, "AI SQL ASSISTANT") {
		t.Errorf("Expected modal title in View, got:\n%s", view)
	}

	// 4. Test Close with Esc
	mod, _ = mod.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if mod.Active {
		t.Errorf("Expected AIModal to close on Esc")
	}
}
