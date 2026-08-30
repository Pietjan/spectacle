//go:build linux

package spectacle

import (
	"log"
	"os"
	"path/filepath"

	"github.com/pietjan/spectacle/internal/native"
)

// sessionManager hands out one WebKitNetworkSession per profile name,
// each with its own data/cache directories — the profiles analogue.
type sessionManager struct {
	dataDir  string
	sessions map[string]*session
}

type session struct {
	handle            uintptr // WebKitNetworkSession*
	dataDir, cacheDir string
	refs              int
	doomed            bool
}

func newSessionManager(dataDir string) *sessionManager {
	return &sessionManager{dataDir: dataDir, sessions: map[string]*session{}}
}

func (m *sessionManager) get(profile string) *session {
	s, ok := m.sessions[profile]
	if !ok {
		name := profile
		if name == "" {
			name = "default"
		}
		s = &session{
			dataDir:  filepath.Join(m.dataDir, "profiles", name, "data"),
			cacheDir: filepath.Join(m.dataDir, "profiles", name, "cache"),
		}
		s.handle = native.WebkitNetworkSessionNew(s.dataDir, s.cacheDir)
		// Favicons never fire without opting in on the data manager.
		dm := native.WebkitNetworkSessionGetWebsiteDataManager(s.handle)
		native.WebkitWebsiteDataManagerSetFaviconsEnabled(dm, 1)
		m.sessions[profile] = s
	}
	s.refs++
	return s
}

// doom marks a profile's data for removal once its webviews close.
func (m *sessionManager) doom(profile string) {
	if s, ok := m.sessions[profile]; ok {
		s.doomed = true
	}
}

func (m *sessionManager) release(profile string) {
	s, ok := m.sessions[profile]
	if !ok {
		return
	}
	s.refs--
	if s.refs > 0 || !s.doomed {
		return
	}
	delete(m.sessions, profile)
	native.GObjectUnref(s.handle)
	// WebKit's network process tears down asynchronously and flushes
	// data AFTER the session is unreffed, resurrecting the directories —
	// so sweep now and again after it has had time to finish.
	root := filepath.Dir(s.dataDir)
	remove := func() {
		if err := os.RemoveAll(root); err != nil {
			log.Printf("linux: profile cleanup %s: %v", root, err)
		}
	}
	remove()
	native.ScheduleTimeout(2500, remove)
}
