package utils

import (
	"os"
	"os/exec"
	"syscall"
)

func OpenEditorWithContent(initial string) (string, error) {
	ed := os.Getenv("EDITOR")
	if ed == "" {
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

	if fd, err := syscall.Dup(int(os.Stdin.Fd())); err == nil {
		cmd.Stdin = os.NewFile(uintptr(fd), "/dev/stdin")
	}
	if fd, err := syscall.Dup(int(os.Stdout.Fd())); err == nil {
		cmd.Stdout = os.NewFile(uintptr(fd), "/dev/stdout")
	}
	if fd, err := syscall.Dup(int(os.Stderr.Fd())); err == nil {
		cmd.Stderr = os.NewFile(uintptr(fd), "/dev/stderr")
	}
	if err := cmd.Run(); err != nil {
		return "", err
	}

	b, err := os.ReadFile(tmpName)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
