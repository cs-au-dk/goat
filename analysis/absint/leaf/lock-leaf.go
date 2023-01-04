package leaf

import (
	"github.com/cs-au-dk/goat/analysis/cfg"
	loc "github.com/cs-au-dk/goat/analysis/location"
)

type (
	// MuLock is a communication leaf where a lock may acquire the lock
	// allocated at the Loc. The CF-node set as the predecessor to a MuLock
	// leaf represents the source location.
	MuLock struct {
		cfg.Synthetic
		Loc loc.Location
	}
	// MuUnlock is a communication leaf where a lock may release the lock
	// allocated at the Loc. The CF-node set as the predecessor to a MuUnlock
	// leaf represents the source location.
	MuUnlock struct {
		cfg.Synthetic
		Loc loc.Location
	}
	// RWMuRLock is a communication leaf where a lock may acquire the read lock
	// allocated at the Loc. The CF-node set as the predecessor to a RWMuRLock
	// leaf represents the source location.
	RWMuRLock struct {
		cfg.Synthetic
		Loc loc.Location
	}
	// RWMuRUnlock is a communication leaf where a lock may release the lock
	// allocated at the Loc. The CF-node set as the predecessor to a RWMuRUnlock
	// leaf represents the source location.
	RWMuRUnlock struct {
		cfg.Synthetic
		Loc loc.Location
	}
)
