package cockroachdb

import (
	"errors"
	"github.com/decred/contractor-mgmt/cmswww/database"
)

var (
	// ErrInvalidType indicates that the given input could not be downcast
	// to the expected type
	ErrInvalidType = errors.New("invalid type")
)

func getIdentityIdx(idToRemove *Identity, ids []*Identity) int {
	for k, v := range ids {
		if v.EncodedKey() == idToRemove.EncodedKey() {
			return k
		}
	}

	return -1
}

func remove(arr []*Identity, idx int) []*Identity {
	return append(arr[:idx], arr[idx+1:]...)
}

func databaseUserToUser(dbu database.User) (*User, error) {
	u, ok := dbu.(*User)
	if !ok {
		return nil, ErrInvalidType
	}
	return u, nil
}

func databaseIdentityToIdentity(dbid database.Identity) (*Identity, error) {
	id, ok := dbid.(*Identity)
	if !ok {
		return nil, ErrInvalidType
	}
	return id, nil
}

func identitiesToDatabaseIdentities(ids []*Identity) []database.Identity {
	dbids := make([]database.Identity, len(ids), len(ids))
	for i := range ids {
		dbids[i] = ids[i]
	}
	return dbids
}
