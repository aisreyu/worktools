package services

import (
	"strings"

	"github.com/aisreyu/worktools/internal/store"
	"github.com/aisreyu/worktools/internal/vault"
)

// NoteService manages the multi-record notepad.
type NoteService struct {
	v *vault.Vault
}

// NewNoteService wires the service to a vault.
func NewNoteService(v *vault.Vault) *NoteService { return &NoteService{v: v} }

// List returns all notes.
func (s *NoteService) List() ([]store.NoteEntry, error) {
	if s.v.Locked() {
		return nil, vault.ErrLocked
	}
	return s.v.ListNotes()
}

// Save creates or updates a note.
func (s *NoteService) Save(e store.NoteEntry) (int64, error) {
	if strings.TrimSpace(e.Title) == "" && strings.TrimSpace(e.Content) == "" {
		return 0, errText("title or content is required")
	}
	if strings.TrimSpace(e.Title) == "" {
		// Derive a short title from the content.
		c := strings.TrimSpace(e.Content)
		if len(c) > 40 {
			c = c[:40]
		}
		e.Title = strings.ReplaceAll(strings.ReplaceAll(c, "\n", " "), "\r", " ")
	}
	return s.v.SaveNote(e)
}

// Delete removes a note.
func (s *NoteService) Delete(id int64) error {
	return s.v.DeleteNote(id)
}
