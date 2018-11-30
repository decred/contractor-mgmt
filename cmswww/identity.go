package main

import (
	"bufio"
	"os"
	"path/filepath"

	"github.com/decred/politeia/util"
)

// Fetch remote identity
func (c *cmswww) RemoteIdentity() error {
	id, err := util.RemoteIdentity(false, c.cfg.RPCHost, c.cfg.RPCCert)
	if err != nil {
		return err
	}

	// Pretty print identity.
	log.Infof("Identity fetched from politeiad")
	log.Infof("Key        : %x", id.Key)
	log.Infof("Fingerprint: %v", id.Fingerprint())

	if c.cfg.Interactive != allowInteractive {
		// Ask user if we like this identity
		log.Infof("Save to %v or ctrl-c to abort", c.cfg.RPCIdentityFile)
		scanner := bufio.NewScanner(os.Stdin)
		scanner.Scan()
		if err = scanner.Err(); err != nil {
			return err
		}
	} else {
		log.Infof("Saving identity to %v", c.cfg.RPCIdentityFile)
	}

	// Save identity
	err = os.MkdirAll(filepath.Dir(c.cfg.RPCIdentityFile), 0700)
	if err != nil {
		return err
	}
	err = id.SavePublicIdentity(c.cfg.RPCIdentityFile)
	if err != nil {
		return err
	}
	log.Infof("Identity saved to: %v", c.cfg.RPCIdentityFile)

	return nil
}
