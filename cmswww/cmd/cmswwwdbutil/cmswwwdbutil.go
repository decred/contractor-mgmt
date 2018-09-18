package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/davecgh/go-spew/spew"
	"github.com/decred/dcrd/chaincfg"
	"github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"

	"github.com/decred/contractor-mgmt/cmswww/database"
	"github.com/decred/contractor-mgmt/cmswww/database/cockroachdb"
	"github.com/decred/contractor-mgmt/cmswww/sharedconfig"
)

var (
	understandTheRisksMagicStr = "i-understand-the-risks-of-this-action"
	createAdminUser            = flag.Bool("createadmin", false, "Create an admin user. Parameters: <email> <username> <password>")
	dataDir                    = flag.String("datadir", sharedconfig.DefaultDataDir, "Specify the cmswww data directory.")
	dbName                     = flag.String("dbname", sharedconfig.DefaultDBName, "Specify the database name.")
	dbUsername                 = flag.String("dbusername", sharedconfig.DefaultDBUsername, "Specify the database username.")
	dbHost                     = flag.String("dbhost", sharedconfig.DefaultDBHost, "Specify the database host.")
	dumpDb                     = flag.Bool("dump", false, "Dump the entire users table contents or contents for a specific user. Parameters: [email]")
	deleteData                 = flag.Bool("deletedata", false, "Drops all tables in the cmswww database. Parameters: \""+understandTheRisksMagicStr+"\"")
	testnet                    = flag.Bool("testnet", false, "Whether to check the testnet database or not.")
	dbDir                      = ""
	db                         database.Database
)

func dumpUser(user *database.User) {
	fmt.Printf("Key    : %v\n", hex.EncodeToString([]byte(user.Email)))
	fmt.Printf("Record : %v", spew.Sdump(*user))
	fmt.Printf("---------------------------------------\n")
}

func dumpUserAction() error {
	// If email is provided, only dump that user.
	args := flag.Args()
	if len(args) == 1 {
		user, err := db.GetUserByEmail(args[0])
		if err != nil {
			if err == database.ErrUserNotFound {
				return fmt.Errorf("user with email %v not found in the database",
					user.Email)
			}
			return err
		}

		fmt.Printf("---------------------------------------\n")
		dumpUser(user)
		return nil
	}

	fmt.Printf("---------------------------------------\n")
	return db.AllUsers(func(user *database.User) {
		dumpUser(user)
	})
}

func deleteDataAction() error {
	args := flag.Args()
	if len(args) < 1 {
		flag.Usage()
		return nil
	}

	if args[0] != understandTheRisksMagicStr {
		flag.Usage()
		return nil
	}

	return db.DeleteAllData()
}

func createAdminUserAction() error {
	args := flag.Args()
	if len(args) < 3 {
		flag.Usage()
		return nil
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(args[2]),
		bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	user := &database.User{}
	user.Email = args[0]
	user.Username = args[1]
	user.HashedPassword = hashedPassword
	user.Admin = true

	if err = db.CreateUser(user); err != nil {
		pqErr, ok := err.(*pq.Error)
		if !ok {
			return err
		}

		switch pqErr.Code {
		case pq.ErrorCode("23505"):
			return fmt.Errorf("user already exists: %v", pqErr.Message)
		default:
			return fmt.Errorf("pq err: %v", pqErr.Code)
		}
	}

	fmt.Printf("Admin user with email %v created\n", user.Email)
	return nil
}

func _main() error {
	flag.Parse()

	var netName string
	if *testnet {
		netName = chaincfg.TestNet3Params.Name
	} else {
		netName = chaincfg.MainNetParams.Name
	}

	var err error
	db, err = cockroachdb.New(filepath.Join(*dataDir, netName), *dbName,
		*dbUsername, *dbHost)
	if err != nil {
		return err
	}
	defer db.Close()

	if *createAdminUser {
		if err := createAdminUserAction(); err != nil {
			return err
		}
	} else if *dumpDb {
		if err := dumpUserAction(); err != nil {
			return err
		}
	} else if *deleteData {
		if err := deleteDataAction(); err != nil {
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
