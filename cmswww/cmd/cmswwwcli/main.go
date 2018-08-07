package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	flags "github.com/jessevdk/go-flags"

	"github.com/decred/contractor-mgmt/cmswww/cmd/cmswwwcli/client"
	"github.com/decred/contractor-mgmt/cmswww/cmd/cmswwwcli/commands"
	"github.com/decred/contractor-mgmt/cmswww/cmd/cmswwwcli/config"
)

func listenForInput() error {
	if !config.JSONOut {
		fmt.Printf("Starting terminal, press ctrl+c to exit.\n\n")
	}

	parser := flags.NewParser(&commands.Opts, flags.Default)
	reader := bufio.NewReader(os.Stdin)
	for {
		if !config.JSONOut {
			if config.LoggedInUser != nil {
				fmt.Printf("%v> ", config.LoggedInUser.Username)
			} else {
				fmt.Printf("> ")
			}
		}
		text, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}

		text = strings.TrimSpace(text)
		_, err = parser.ParseArgs(strings.Fields(text))
		if err != nil {
			// Ignore error
		}
	}

	return nil
}

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

	// configure client
	// TODO: move
	if len(config.Cookies) != 0 {
		commands.Ctx.SetCookies(config.Host, config.Cookies)
	}
	commands.Ctx.Csrf = config.CsrfToken

	version := commands.VersionCmd{}
	err = version.Execute(nil)
	if err != nil {
		return err
	}

	// start listening for input
	err = listenForInput()
	if err != nil {
		return err
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
