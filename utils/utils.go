package utils

import (
	"os"
	"os/exec"
)

func OpenEditorWithContent(initial string) (string, error) {
	ed := os.Getenv("EDITOR")
	if ed == "" {
		// prefer nvim if available
		if p, err := exec.LookPath("nvim"); err == nil {
			ed = p
		} else if p, err := exec.LookPath("vi"); err == nil {
			ed = p
		} else {
			ed = "ed"
		}
	}

	tmp, err := os.CreateTemp("", "blue-note-*.md")
	if err != nil {
		return "", err
	}
	tmpName := tmp.Name()
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
	}()

	if _, err := tmp.WriteString(initial); err != nil {
		return "", err
	}
	if err := tmp.Sync(); err != nil {
		// ignore
	}

	cmd := exec.Command(ed, tmpName)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return "", err
	}

	b, err := os.ReadFile(tmpName)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
