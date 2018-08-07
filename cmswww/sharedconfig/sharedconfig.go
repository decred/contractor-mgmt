package sharedconfig

import (
	"path/filepath"

	"github.com/decred/dcrd/dcrutil"
)

const (
	DefaultConfigFilename = "cmswww.conf"
	DefaultDataDirname    = "data"
)

var (
	// DefaultHomeDir points to cmswww's home directory for configuration and data.
	DefaultHomeDir = dcrutil.AppDataDir("cmswww", false)

	// DefaultConfigFile points to cmswww's default config file.
	DefaultConfigFile = filepath.Join(DefaultHomeDir, DefaultConfigFilename)

	// DefaultDataDir points to cmswww's default data directory.
	DefaultDataDir = filepath.Join(DefaultHomeDir, DefaultDataDirname)
)
