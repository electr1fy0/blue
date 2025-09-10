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
