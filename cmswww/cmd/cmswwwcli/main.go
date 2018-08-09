package main

import (
	"fmt"
	"os"

	flags "github.com/jessevdk/go-flags"

	"github.com/decred/contractor-mgmt/cmswww/cmd/cmswwwcli/client"
	"github.com/decred/contractor-mgmt/cmswww/cmd/cmswwwcli/commands"
	"github.com/decred/contractor-mgmt/cmswww/cmd/cmswwwcli/config"
)

func _main() error {
	err := config.Load()
	if err != nil {
		return err
	}

	// create new http client
	commands.Ctx, err = client.NewClient(true)
	if err != nil {
		return err
	}

	if len(config.Cookies) != 0 {
		commands.Ctx.SetCookies(config.Host, config.Cookies)
	}

	// Parse and handle the command.
	commands.SetupOptsFunctions()
	var parser = flags.NewParser(&commands.Opts, flags.Default)
	if _, err := parser.Parse(); err != nil {
		if flagsErr, ok := err.(*flags.Error); ok && flagsErr.Type == flags.ErrHelp {
			return nil
		}
		return err
	}

	if config.JSONOutput {
		fmt.Printf("%v\n", commands.Ctx.LastCommandOutput)
	}

	return nil
}

func main() {
	err := _main()
	if err != nil {
		//fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}
