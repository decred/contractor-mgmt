# cmswwwdataload

cmswwwdataload is a tool that loads basic data into Politeia to help
speed up full end-to-end testing. It will automatically start and stop
politeiad and cmswww, and utilize the cmswwwcli tool to create the following:

* Admin user
* Regular paid user
* Regular unpaid user
* A proposal for each state (public, censored, etc)
* A couple comments on the public proposal

Because it starts and stops politeiad and cmswww automatically, you
will need to ensure that those servers are shut down before running this tool.
It will run the servers with some fixed configuration, although some default
configuration is required, so you should have politeiad.conf and cmswww.conf
already set up.

When running cmswwwdataload twice, the second time will fail because it
can't create duplicate users.

## Usage

This tool doesn't require any arguments, but you can specify the following options.
Additionally, you can specify these options in a `cmswwwdataload.conf` file,
which should be located under `/Users/<username>/Library/Application Support/Cmswww/dataload/`.

```
     --adminemail   admin email address
     --adminuser    admin username
     --adminpass    admin password
     --paidemail    paid user email address
     --paiduser     paid user username
     --paidpass     paid user password
     --unpaidemail  unpaid user email address
     --unpaiduser   unpaid user username
     --unpaidpass   unpaid user password
     --deletedata   before loading the data, delete all existing data
     --debuglevel   the debug level to set when starting politeiad and cmswww
                    server; the servers' log output is stored in the data directory
     --datadir      specify a different directory to store log files
     --configfile   specify a different .conf file for config options
 -v, --verbose      verbose output
```

Example:

```
cmswwwdataload --verbose
```

## Troubleshooting

If you encounter an error while running cmswwwdataload, it's possible that
some program this depends on is out of date. Before opening a Github issue,
make sure to pull the latest from master and build all programs:

    cd $GOPATH/src/github.com/decred/politeia
    dep ensure && go install -v ./...
