package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/davecgh/go-spew/spew"
	"github.com/decred/contractor-mgmt/cmswww/database/localdb"
	"github.com/decred/contractor-mgmt/cmswww/sharedconfig"
	"github.com/decred/dcrd/chaincfg"
	"golang.org/x/crypto/bcrypt"

	"github.com/decred/contractor-mgmt/cmswww/database"
)

var (
	createAdminUser = flag.Bool("createadmin", false, "Create an admin user. Parameters: <email> <password>")
	dataDir         = flag.String("datadir", sharedCfg.DefaultDataDir, "Specify the cmswww data directory.")
	dumpDb          = flag.Bool("dump", false, "Dump the entire cmswww database contents or contents for a specific user. Parameters: [email]")
	testnet         = flag.Bool("testnet", false, "Whether to check the testnet database or not.")
	dbDir           = ""
	db              database.Database
)

func dumpUser(user database.User) {
	fmt.Printf("Key    : %v\n", hex.EncodeToString([]byte(user.Email)))
	fmt.Printf("Record : %v", spew.Sdump(user))
	fmt.Printf("---------------------------------------\n")
}

func dumpAction() error {
	// If email is provided, only dump that user.
	args := flag.Args()
	if len(args) == 1 {
		user, err := db.UserGet(args[0])
		if err != nil {
			if err == database.ErrUserNotFound {
				return fmt.Errorf("user with email %v not found in the database",
					user.Email)
			}
			return err
		}

		fmt.Printf("---------------------------------------\n")
		dumpUser(*user)
		return nil
	}

	fmt.Printf("---------------------------------------\n")
	return db.AllUsers(func(user *database.User) {
		dumpUser(*user)
	})
}

func createAdminUserAction() error {
	args := flag.Args()
	if len(args) < 2 {
		flag.Usage()
		return nil
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(args[1]),
		bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	user := database.User{
		Email:          args[0],
		Username:       "admin",
		HashedPassword: hashedPassword,
		Admin:          true,
	}

	_, err = db.UserGet(user.Email)
	if err == nil {
		return fmt.Errorf("user with email %v already found in the database",
			user.Email)
	}

	if err = db.UserNew(user); err != nil {
		return err
	}

	fmt.Printf("Admin user with email %v created\n", user.Email)
	return nil
}

func _main() error {
	flag.Parse()

	var netName string
	if *testnet {
		netName = chaincfg.TestNet2Params.Name
	} else {
		netName = chaincfg.MainNetParams.Name
	}

	var err error
	db, err = localdb.New(filepath.Join(*dataDir, netName))
	if err != nil {
		return err
	}
	defer db.Close()

	if *createAdminUser {
		if err := createAdminUserAction(); err != nil {
			return err
		}
	} else if *dumpDb {
		if err := dumpAction(); err != nil {
			return err
		}
	} else {
		flag.Usage()
	}

	return nil
}

func main() {
	err := _main()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}
