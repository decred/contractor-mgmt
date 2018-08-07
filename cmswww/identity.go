package main

import (
	"bufio"
	"encoding/hex"
	"os"
	"path/filepath"
	"strconv"

	"github.com/decred/politeia/util"

	"github.com/decred/contractor-mgmt/cmswww/database"
)

// initUserPubkeys initializes the userPubkeys map with all the pubkey-userid
// associations that are found in the database.
//
// This function must be called WITHOUT the lock held.
func (c *cmswww) InitUserPubkeys() error {
	c.Lock()
	defer c.Unlock()

	return c.db.AllUsers(func(u *database.User) {
		id := strconv.FormatUint(u.ID, 10)
		for _, v := range u.Identities {
			key := hex.EncodeToString(v.Key[:])
			c.userPubkeys[key] = id
		}
	})
}

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

// SetUserPubkeyAssociaton associates a public key with a user id in
// the userPubkeys cache.
//
// This function must be called WITHOUT the lock held.
func (c *cmswww) SetUserPubkeyAssociaton(user *database.User, publicKey string) {
	c.Lock()
	defer c.Unlock()

	c.userPubkeys[publicKey] = strconv.FormatUint(user.ID, 10)
}

// RemoveUserPubkeyAssociaton removes a public key from the
// userPubkeys cache.
//
// This function must be called WITHOUT the lock held.
func (c *cmswww) RemoveUserPubkeyAssociaton(user *database.User, publicKey string) {
	c.Lock()
	defer c.Unlock()

	delete(c.userPubkeys, publicKey)
}
