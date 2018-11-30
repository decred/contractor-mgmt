# cmswwwdbutil

`cmswwwdbutil` is a convenience tool for interacting with the cmswww
database.


## Usage

You can specify the following configuration options:

```
-testnet
Whether to interact with the testnet or mainnet database

-dbhost <host>
Specify the database host. Default: localhost:26257

-dbname <name>
Specify the database name. Default: cmswww

-dbusername <username>
Specify the database username. Default: cmswwwuser

-datadir <dir>
Specify the directory where the database is stored. Default: ~/cmswww/data
```

And the following actions:

```
-dump [email]
Print the contents of the entire set of users to the console, or the
contents of a specific user, if email is provided.

-createadmin <email> <username> <password>
Creates an admin user.

-deletedata i-understand-the-risks-of-this-action
Drops all tables in the database.
```

Example:

```
cmswwwdbutil -createadmin user@example.com user password
```
