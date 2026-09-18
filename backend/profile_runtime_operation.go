package backend

import (
	"strings"
	"sync"
)

// lockProfileRuntimeOperation serializes lifecycle operations for one profile
// without blocking unrelated profiles behind the manager-wide mutex.
func (a *App) lockProfileRuntimeOperation(profileID string) func() {
	if a == nil {
		return func() {}
	}
	profileID = strings.TrimSpace(profileID)
	a.profileRuntimeOpsMu.Lock()
	if a.profileRuntimeOps == nil {
		a.profileRuntimeOps = make(map[string]*sync.Mutex)
	}
	lock := a.profileRuntimeOps[profileID]
	if lock == nil {
		lock = &sync.Mutex{}
		a.profileRuntimeOps[profileID] = lock
	}
	a.profileRuntimeOpsMu.Unlock()

	lock.Lock()
	return lock.Unlock
}
