package model

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/electr1fy0/blue/storage"
)

func (m *Model) changePassword(newPassword string) error {
	if m.nb == nil {
		return fmt.Errorf("no notebook loaded")
	}
	// Try saving with new password
	if err := storage.SaveNotebook(m.nb, newPassword); err != nil {
		return err
	}
	// If success, update in-memory password
	m.password = newPassword
	m.status = "Password changed."
	return nil
}

func (m *Model) exportNotes() error {
	exportDir := fmt.Sprintf("blue_export_%d", time.Now().Unix())
	if err := os.MkdirAll(exportDir, 0755); err != nil {
		return err
	}

	count := 0
	for title, note := range m.nb.Notes {
		filename := strings.ReplaceAll(title, "/", "_") + ".md"
		path := filepath.Join(exportDir, filename)
		if err := os.WriteFile(path, []byte(note.Content), 0644); err != nil {
			return err
		}
		count++
	}

	m.status = fmt.Sprintf("Exported %d notes to %s/", count, exportDir)
	return nil
}
