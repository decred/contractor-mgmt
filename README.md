# contractor-mgmt
This is a contractor management system written in Go that uses politeiad
as a backend. It stores invoices, which are submitted by contractors and then
approved and paid by administrators.

## Components

### Core components

* cmswww - Web server; depends on politeiad.

### Tools and reference clients

* [cmswwwcli](https://github.com/decred/contractor-mgmt/tree/master/cmswww/cmd/cmswwwcli) - Command-line tool for interacting with cmswww.
* [cmswwwdbutil](https://github.com/decred/contractor-mgmt/tree/master/cmswww/cmd/cmswwwdbutil) - Tool for debugging and creating admin users within the cmswww database.
* [cmswwwdataload](https://github.com/decred/contractor-mgmt/tree/master/cmswww/cmd/cmswwwdataload) - Tool using cmswwwcli to load a basic dataset into cmswww.

**Note:** cmswww does not provide HTML output.  It strictly handles the
JSON REST RPC commands only.

## Development

#### 1. Install [Go](https://golang.org/doc/install) version 1.11 or greater, and [Git](https://git-scm.com/downloads).

Make sure each of these are in the PATH.

#### 2. Setup and start CockroachDB.

CockroachDB is used by cmswww as storage for users, invoices and other data. Some data,
such as invoices, are pulled from politeiad when cmswww starts and are added to the database.

To set up CockroachDB for use with cmswww, you can either do this manually or, on Linux, using go-task if you prefer.

##### Manual method

  1. Install [CockroachDB](https://www.cockroachlabs.com/docs/stable/install-cockroachdb-windows.html).

  2. Create the root and cmswwwuser clients, the cmswww database, and a node on localhost.

     Replace `<install dir>` with the installation location for CockroachDB on your machine:
                   
         cd <install dir>
         
         mkdir -p ${HOME}/.cmswww/data/testnet3/cockroachdb

         cockroach cert create-ca --certs-dir=${HOME}/.cmswww/data/testnet3/cockroachdb --ca-key=<install dir>/ca.key --allow-ca-key-reuse

         cockroach cert create-client root --certs-dir=${HOME}/.cmswww/data/testnet3/cockroachdb --ca-key=<install dir>/ca.key

         cockroach cert create-node localhost --certs-dir=${HOME}/.cmswww/data/testnet3/cockroachdb --ca-key=<install dir>/ca.key

         cockroach start --host=localhost --http-host=localhost --certs-dir=${HOME}/.cmswww/data/testnet3/cockroachdb

         cockroach user set cmswwwuser --certs-dir=${HOME}/.cmswww/data/testnet3/cockroachdb

         cockroach cert create-client cmswwwuser --certs-dir=${HOME}/.cmswww/data/testnet3/cockroachdb --ca-key=<install dir>/ca.key

         cockroach sql --certs-dir=${HOME}/.cmswww/data/testnet3/cockroachdb -e 'CREATE DATABASE cmswww'

         cockroach sql --certs-dir=${HOME}/.cmswww/data/testnet3/cockroachdb -e 'GRANT ALL ON DATABASE cmswww TO cmswwwuser'

         cockroach sql --user=cmswwwuser --certs-dir=${HOME}/.cmswww/data/testnet3/cockroachdb -e 'GRANT ALL ON DATABASE cmswww TO cmswwwuser'

   3. Start CockroachDB.

          cd <install dir>
          cockroach start --host=localhost --http-host=localhost --certs-dir=${HOME}/.cmswww/data/testnet3/cockroachdb

##### Task method

  1.  Ensure that [task](https://taskfile.org) is installed)

  2.  Run ```task init_cdb```

This will run through all the steps described under the manual method above, 
choosing default folders for the data.  Running ```task init_cdb``` a second
time will delete the database and rerun steps again.

Run ```task -l``` to see other commands available.  Some additional commands
may require that politeiad or cmswww are available on your system.

#### 2. Clone this repository and [decred/politeia](https://github.com/decred/politeia).

#### 3. Setup configuration files:

Both politeiad and cmswww have configuration files that you should set up to
make execution easier. You should create the configuration files under the
following paths:

* **macOS**

   ```
   /Users/<username>/Library/Application Support/Politeiad/politeiad.conf
   /Users/<username>/Library/Application Support/Cmswww/cmswww.conf
   ```

* **Windows**

   ```
   C:\Users\<username>\AppData\Local\Politeiad/politeiad.conf
   C:\Users\<username>\AppData\Local\Cmswww/cmswww.conf
   ```

* **Ubuntu**

   ```
   ~/.politeiad/politeiad.conf
   ~/.cmswww/cmswww.conf
   ```

Copy and change the [`sample-cmswww.conf`](https://github.com/decred/contractor-mgmt/blob/master/cmswww/sample-cmswww.conf)
and [`sample-politeiad.conf`](https://github.com/decred/politeia/blob/master/politeiad/sample-politeiad.conf) files.

You can also use the following default configurations:

**politeiad.conf**:

    rpcuser=user
    rpcpass=pass
    testnet=true


**cmswww.conf**:

    rpchost=127.0.0.1
    rpcuser=user
    rpcpass=pass
    rpccert="/Users/<username>/Library/Application Support/Politeiad/https.cert"
    testnet=true

**Things to note:**

* The `rpccert` path is referencing a macOS path. See above for
more OS paths.

* cmswww uses an email server to send verification codes for
things like new user registration, and those settings are also configured within
 `cmswww.conf`. The current code should work with most SSL-based SMTP servers
(but not TLS) using username and password as authentication.

#### 4. Build the programs:

```
cd $GOPATH/src/github.com/decred/politeia
GO111MODULE=on go mod vendor && go install -v ./...
cd $GOPATH/src/github.com/decred/contractor-mgmt
GO111MODULE=on go mod vendor && go install -v ./...
```

#### 5. Start the politeiad server by running on your terminal:

    politeiad

#### 6. Download politeiad's identity to cmswww:

    cmswww --fetchidentity

Accept politeiad's identity by pressing <kbd>Enter</kbd>.

The result should look something like this:

```
2018-08-01 22:48:48.468 [INF] CWWW: Identity fetched from politeiad
2018-08-01 22:48:48.468 [INF] CWWW: Key        : 331819226de0270d0c997749ce9f2b56bc5aed110f57faef8d381129e7ee6d26
2018-08-01 22:48:48.468 [INF] CWWW: Fingerprint: MxgZIm3gJw0MmXdJzp8rVrxa7REPV/rvjTgRKefubSY=
2018-08-01 22:48:48.468 [INF] CWWW: Save to /Users/<username>/Library/Application Support/Cmswww/identity.json or ctrl-c to abort

2018-08-01 22:49:53.929 [INF] CWWW: Identity saved to: /Users/<username>/Library/Application Support/Cmswww/identity.json
```

#### 7. Start the cmswww server by running on your terminal:

    cmswww

At this point, you can:

* Use the [cmswwwcli](https://github.com/decred/contractor-mgmt/tree/master/cmswww/cmd/cmswwwcli) tool to interact with cmswww.
* Use the [politeia](https://github.com/decred/politeia/tree/master/politeiad/cmd/politeia) tool to interact directly with politeiad.
* Use any other tools or clients that are listed above.


### Further information

## Library and interfaces

* `cmswww/api/v1` - JSON REST API for cmswww clients.
