package player

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	librespot "github.com/devgianlu/go-librespot"
	"github.com/zalando/go-keyring"
)

const keyringService = "spotimy"

// oldKeyringService is where the login was kept when the app was called
// personal-spotify; Load moves it over.
const oldKeyringService = "personal-spotify"

type credentials struct {
	Username string `json:"username"`
	Data     []byte `json:"data"`
}

// stateStore persists go-librespot's AppState. The login credentials go to the
// OS keyring; everything else (device id, volume, event manager state) goes to
// a 0600 JSON file. If the keyring is unavailable the credentials are kept in
// that file too, still 0600.
type stateStore struct {
	path     string
	lastCred credentials // last value written to the keyring, to skip D-Bus round trips
}

// fileState mirrors AppState without its mutex so it can be marshalled.
type fileState struct {
	DeviceId     string          `json:"device_id"`
	EventManager json.RawMessage `json:"event_manager,omitempty"`
	LastVolume   *uint32         `json:"last_volume,omitempty"`
	Credentials  *credentials    `json:"credentials,omitempty"` // only when the keyring failed
}

func newStateStore(dir string) *stateStore {
	return &stateStore{path: filepath.Join(dir, "state.json")}
}

func (s *stateStore) Load() (*librespot.AppState, error) {
	st := &librespot.AppState{}

	var fs fileState
	data, err := os.ReadFile(s.path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	} else if err == nil {
		if err := json.Unmarshal(data, &fs); err != nil {
			return nil, fmt.Errorf("parsing %s: %w", s.path, err)
		}
	}
	st.DeviceId, st.EventManager, st.LastVolume = fs.DeviceId, fs.EventManager, fs.LastVolume

	cred := fs.Credentials
	raw, err := keyring.Get(keyringService, "credentials")
	if errors.Is(err, keyring.ErrNotFound) {
		if old, oerr := keyring.Get(oldKeyringService, "credentials"); oerr == nil && keyring.Set(keyringService, "credentials", old) == nil {
			_ = keyring.Delete(oldKeyringService, "credentials")
			raw, err = old, nil
		}
	}
	if err == nil {
		var c credentials
		if json.Unmarshal([]byte(raw), &c) == nil {
			cred = &c
			s.lastCred = c
		}
	}
	if cred != nil {
		st.Credentials.Username, st.Credentials.Data = cred.Username, cred.Data
	}
	return st, nil
}

func (s *stateStore) Save(st *librespot.AppState) error {
	st.Lock()
	fs := fileState{DeviceId: st.DeviceId, EventManager: st.EventManager, LastVolume: st.LastVolume}
	cred := credentials{Username: st.Credentials.Username, Data: st.Credentials.Data}
	st.Unlock()

	if len(cred.Data) > 0 && (cred.Username != s.lastCred.Username || string(cred.Data) != string(s.lastCred.Data)) {
		raw, _ := json.Marshal(cred)
		if err := keyring.Set(keyringService, "credentials", string(raw)); err != nil {
			fs.Credentials = &cred
		} else {
			s.lastCred = cred
		}
	}

	data, err := json.Marshal(fs)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

// wipe deletes the credentials from the keyring and the state file.
func (s *stateStore) wipe() error {
	s.lastCred = credentials{}
	err := keyring.Delete(keyringService, "credentials")
	if errors.Is(err, keyring.ErrNotFound) {
		err = nil
	}
	_ = keyring.Delete(oldKeyringService, "credentials")
	if rmErr := os.Remove(s.path); rmErr != nil && !errors.Is(rmErr, os.ErrNotExist) {
		err = errors.Join(err, rmErr)
	}
	return err
}

// StoredCredentials returns the persisted login, for dev tools that need a
// token without running the whole player.
func StoredCredentials(configDir string) (username string, data []byte, deviceID string, err error) {
	st, err := newStateStore(configDir).Load()
	if err != nil {
		return "", nil, "", err
	}
	if len(st.Credentials.Data) == 0 {
		return "", nil, "", errors.New("not logged in; start the app once")
	}
	return st.Credentials.Username, st.Credentials.Data, st.DeviceId, nil
}
