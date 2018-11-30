# cmswwwcli

`cmswwwcli` is a command line tool that allows you to interact with the Politeia API.

## Available Commands

Execute `cmswwwcli --help` for a list of available commands.

## Flows

### All users

#### Login

```
cmswwwcli login <email> <password>
```

#### Logout

```
cmswwwcli logout
```

### Admins

#### Invite a contractor to register

```
cmswwwcli invite <contractor email>
```

#### Generate a list of unreviewed invoices

```
cmswwwcli reviewinvoices dec 2018 > 2018-12_reviews.txt
```

#### Approve or reject an invoice

```
cmswwwcli setinvoicestatus <invoice token> approved
or
cmswwwcli setinvoicestatus <invoice token> rejected <reason for rejection>
```

#### Generate a list of approved invoices (to be paid)

```
cmswwwcli payinvoices dec 2018 <DCR-USD rate> > 2018-12_payouts.txt
```

### Contractors

#### Register

Use the token from your invitation email to register and follow the instructions:

```
cmswwwcli register <email> <token>

Create a username: <username>
Create a password: <password>
Enter your full name: <name>
Enter your location: <location>
Enter the extended public key for your payment account: <extended pubkey>
```

#### Submit an invoice

Invoices must be in the following CSV format:

```
# 2018-12
# Type of work, Subtype of work, Description of work, Link to Politeia proposal, Hours worked, Total cost (in USD)
Development,,decred/politeia issue#36,,4,160
Development,,decred/politeia issue#38,,3,120
...
```

You can either create the file manually, or have the CLI create and update a file
for you, using the `logwork` command:

```
cmswwwcli logwork dec 2018

Type of work: Development
Subtype of work (optional):
Description of work: decred/politeia issue#36
Politeia proposal (optional):
Hours worked: 4
Total cost (in USD): 160
Work logged successfully.
```

When you're ready to submit the invoice for review, you can either submit the
one created and maintained by the CLI via the `logwork` command:

```
cmswwwcli submitinvoice dec 2018
```

Or submit your own CSV file:

```
cmswwwcli submitinvoice --invoice=<path to invoice CSV>

Invoice submitted successfully! The censorship record has been stored in ~/cmswww/cli/invoices/<email>/submission_record_2018-12_1.json for your future reference.
```

#### Editing a rejected invoice

If your invoice is rejected, you can edit and re-submit it:

```
cmswwwcli editinvoice <invoice token> <path to invoice CSV>

Invoice submitted successfully! The censorship record has been stored in ~/cmswww/cli/invoices/<email>/submission_record_2018-12_2.json for your future reference.
```

#### Setting email notification preferences

Contractors have the ability to get email notifications for changes to their invoices:

```
cmswwwcli edituser --emailnotifications=<num>
```

`<num>` is the result of adding the corresponding numbers for any combination of the following notification types:

* Invoice has been approved: `1`
* Invoice has been rejected: `2`
* Payment received for invoice: `4`

For example, to only get notifications for when your invoices are approved or rejected, you will substitute `3` for `<num>` in the above command.

## Application Options
```
    --host     cmswww host (default: https://127.0.0.1:4443)
    --jsonout  Print JSON
-v, --verbose  Print request and response details

```

**If you're running Politeia locally, you need to make sure to specify the host.**
`$ cmswwwcli --host https://localhost:4443 <command>`

## Help Options
`-h, --help  Show the help message`

View a list of all commands
`$ cmswwwcli -h`

View information about a specific command
`$ cmswwwcli <command> -h`

## Persisting Data Between Commands
`cmswwwcli` stores  user identity data (user's public/private key pair), session cookies, and CSRF tokens in the `AppData/Cmswww/cli/` directory.  This allows you to login with a user and remain logged in between commands.  The user identity data and cookies are segmented by host, allowing you to login and interact with multiple hosts simultaneously.

## Usage

Create a new user.
```
$ cmswwwcli -j --host https://localhost:4443 newuser email@example.com username password --verify --paywall
```
`--verify` will satisfy the email verification requirement for the user.
`--paywall` will use the Decred testnet faucet to satisfy the user registration fee requirement.

**Note: If you use the --paywall flag, you will still need to wait for block confirmations before you'll be allowed to submit proposals.**

Login with the user.
`$ cmswwwcli --host https://localhost:4443 login email@example.com password`

Once logged in, you can submit proposals, comment on proposals, cast votes, or perform any of the other user actions that Politeia allows.

## 403 Error
If you receive a 403 from the Politeia server, it's most likely an issue with the CSRF tokens.  You can fix this by running either the `version` command or by loggin in with a user.

## Proposal Status Codes
Admins can set the status of a proposal with the `setproposalstatus` command.  The proposal status codes are listed below.

```
PropStatusInvalid      0 // Invalid status
PropStatusNotFound     1 // Proposal not found
PropStatusNotReviewed  2 // Proposal has not been reviewed
PropStatusCensored     3 // Proposal has been censored
PropStatusPublic       4 // Proposal is publicly visible
PropStatusLocked       6 // Proposal is locked
```

## User Edit Action Codes
Admin users can edit certain properties of other users with the `useredit` command.  The edit action codes are listed below.

```
UserManageExpireRegisterVerification        1 // Expire new user verification
UserManageExpireUpdateKeyVerification      2 // Expire update key verification
UserManageExpireResetPasswordVerification  3 // Expire reset password verification
UserManageClearUserPayment                 4 // Clear user paywall
UserManageUnlock                           5 // Unlock user
```
