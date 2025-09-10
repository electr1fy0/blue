package storage

import (
	"blue/crypto"
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type Note struct {
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Notebook struct {
	Version int              `json:"version"`
	Notes   map[string]*Note `json:"notes"`
}

func NewNotebook() *Notebook {
	return &Notebook{
		Version: 1,
		Notes:   make(map[string]*Note),
	}
}

func (nb *Notebook) AddNote(note *Note) {
	now := time.Now()
	note.CreatedAt = now
	note.UpdatedAt = now
	nb.Notes[note.Title] = note
}

func (nb *Notebook) GetNote(title string) (*Note, bool) {
	note, exists := nb.Notes[title]
	return note, exists
}

func (nb *Notebook) DeleteNote(title string) bool {
	if _, exists := nb.Notes[title]; exists {
		delete(nb.Notes, title)
		return true
	}
	return false
}

func (nb *Notebook) ListNotes() []string {
	titles := make([]string, 0, len(nb.Notes))
	for title := range nb.Notes {
		titles = append(titles, title)
	}
	return titles
}

func (nb *Notebook) ToJSON() ([]byte, error) {
	return json.Marshal(nb)
}

func FromJSON(data []byte) (*Notebook, error) {
	var nb Notebook
	err := json.Unmarshal(data, &nb)
	if err != nil {
		return nil, err
	}
	return &nb, nil
}

func GetNotebookPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".blue-vault"), nil
}

func NotebookExists() (bool, error) {
	path, err := GetNotebookPath()
	if err != nil {
		return false, err
	}
	_, err = os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func SaveNotebook(nb *Notebook, password string) error {
	jsonData, err := nb.ToJSON()
	if err != nil {
		return err
	}
	encryptedData, err := crypto.Encrypt(jsonData, password)
	if err != nil {
		return err
	}
	encryptedJSON, err := json.Marshal(encryptedData)
	if err != nil {
		return err
	}
	path, err := GetNotebookPath()
	if err != nil {
		return err
	}
	return os.WriteFile(path, encryptedJSON, 0600)
}

func LoadNotebook(password string) (*Notebook, error) {
	path, err := GetNotebookPath()
	if err != nil {
		return nil, err
	}
	encryptedJSON, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var encryptedData crypto.EncryptedData
	if err := json.Unmarshal(encryptedJSON, &encryptedData); err != nil {
		return nil, err
	}
	jsonData, err := crypto.Decrypt(encryptedData, password)
	if err != nil {
		return nil, err
	}
	return FromJSON(jsonData)
}
