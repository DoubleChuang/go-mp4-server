package videoserver

import (
	"encoding/json"
	"os"
	"sync"
)

type totpUser struct {
	Name       string `json:"name"`
	TotpSecret string `json:"totp_secret"`
	Enabled    bool   `json:"enabled"`
}

type totpFile struct {
	Users []totpUser `json:"users"`
}

// TotpStore persists TOTP enrollment state in a JSON file.
type TotpStore struct {
	mu   sync.Mutex
	path string
}

// NewTotpStore returns a store backed by the file at path.
func NewTotpStore(path string) *TotpStore {
	return &TotpStore{path: path}
}

func (s *TotpStore) read() (totpFile, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return totpFile{}, nil
		}
		return totpFile{}, err
	}
	var f totpFile
	if err := json.Unmarshal(data, &f); err != nil {
		return totpFile{}, err
	}
	return f, nil
}

func (s *TotpStore) write(f totpFile) error {
	data, err := json.MarshalIndent(f, "", "    ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

// Enabled reports whether username has 2FA enabled. A missing file means nobody does.
func (s *TotpStore) Enabled(username string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, err := s.read()
	if err != nil {
		return false, err
	}
	for _, u := range f.Users {
		if u.Name == username && u.Enabled {
			return true, nil
		}
	}
	return false, nil
}

// Secret returns the TOTP secret for username, ok=false if not enrolled.
func (s *TotpStore) Secret(username string) (string, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, err := s.read()
	if err != nil {
		return "", false, err
	}
	for _, u := range f.Users {
		if u.Name == username && u.Enabled {
			return u.TotpSecret, true, nil
		}
	}
	return "", false, nil
}

// Save enrolls username with the given secret (enabled=true).
func (s *TotpStore) Save(username, secret string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, err := s.read()
	if err != nil {
		return err
	}
	replaced := false
	for i := range f.Users {
		if f.Users[i].Name == username {
			f.Users[i].TotpSecret = secret
			f.Users[i].Enabled = true
			replaced = true
			break
		}
	}
	if !replaced {
		f.Users = append(f.Users, totpUser{Name: username, TotpSecret: secret, Enabled: true})
	}
	return s.write(f)
}

// Disable removes username's entry entirely.
func (s *TotpStore) Disable(username string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, err := s.read()
	if err != nil {
		return err
	}
	out := f.Users[:0]
	for _, u := range f.Users {
		if u.Name != username {
			out = append(out, u)
		}
	}
	f.Users = out
	return s.write(f)
}