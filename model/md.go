package model

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"

	markdown "github.com/MichaelMure/go-term-markdown"
)

func renderMarkdownToANSI(md string, width int) string {
	if width < 40 {
		width = 40
	}
	out := markdown.Render(md, width-4, 4)
	return string(out)
}

func renderWithGlow(md string) (string, error) {
	glowPath, err := exec.LookPath("glow")
	if err != nil {
		return "", fmt.Errorf("glow not found")
	}

	tmp, err := os.CreateTemp("", "note-*.md")
	if err != nil {
		return "", err
	}
	tmpName := tmp.Name()
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
	}()

	if _, err := tmp.WriteString(md); err != nil {
		tmp.Close()
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}

	cmd := exec.Command(glowPath, "-s", "dark", tmpName)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	cmd.Env = append(os.Environ(), "GLOW_STYLE=dark")

	if err := cmd.Run(); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// // Put this function right after the renderMarkdown function (around line 60-70)
// func (m *Model) renderNoteContent(content string) string {
// 	// Try renderWithGlow with timeout
// 	glowChan := make(chan string, 1)
// 	errChan := make(chan error, 1)

// 	go func() {
// 		if out, err := renderWithGlow(content); err == nil {
// 			glowChan <- out
// 		} else {
// 			errChan <- err
// 		}
// 	}()

// 	select {
// 	case rendered := <-glowChan:
// 		return rendered
// 	case <-errChan:
// 		// Fall through to markdown fallback
// 	case <-time.After(500 * time.Millisecond): // 500ms timeout
// 		// Glow is taking too long, use fallback
// 	}

// 	// Fallback to glamour markdown
// 	if out, err := renderMarkdown(content, m.width); err == nil {
// 		return out
// 	}

// 	// Final fallback
// 	return renderMarkdownToANSI(content, m.width)
// }
