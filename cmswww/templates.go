// Copyright (c) 2018 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

const templateRegisterEmailRaw = `
You are invited to join Decred as a contractor! To complete your registration, you will need to use the following link and register on the CMS site:

https://cms.decred.org

Email: {{.Email}}
Token: {{.Token}}

You will need to complete the rest of the requested information and upon submission you will be fully registered and ready to submit invoices.

Otherwise you can download and build cmswwwcli (from https://github.com/decred/contractor-mgmt/tree/master/cmswww/cmd/cmswwwcli) and execute it as follows:

$ cmswwwcli register {{.Email}} {{.Token}}

Or you can use the follwoing
You are receiving this email because {{.Email}} was invited to join Decred. If you have no knowledge of this invitation, please ignore this email.
`

const templateNewIdentityEmailRaw = `
You have generated a new identity. To verify and start using it, you will need to execute the following:

$ cmswwwcli login {{.Email}} <password>
$ cmswwwcli verifyidentity {{.Token}}

You are receiving this email because a new identity (public key: {{.PublicKey}}) was generated for {{.Email}} on Decred Contractor Management. If you did not perform this action, please notify the administrators.
`

const templateUserLockedResetPasswordRaw = `
Your account was locked due to too many login attempts. You need to reset your password in order to unlock your account by executing the following:

$ cmswwwcli resetpassword {{.Email}}

You are receiving this email because someone made too many login attempts for {{.Email}} on Decred Contractor Management. If that was not you, please notify the administrators.
`

const templateResetPasswordEmailRaw = `
You have reset your password. To verify your password reset, you will need to execute the following:

$ cmswwwcli resetpassword {{.Email}} --token={{.Token}} --newpassword=<your new password>

You are receiving this email because the password has been reset for {{.Email}} on Decred Contractor Management. If you did not perform this action, please notify the administrators.
`

const templateUpdateExtendedPublicKeyEmailRaw = `
To update your extended public key, you will need to execute the following:

$ cmswwwcli updatexpublickey --token={{.Token}} --xpublickey=<your extended public key>

You are receiving this email because it has been requested to update the extended public key for {{.Email}} on Decred Contractor Management. If you did not perform this action, please notify the administrators.
`

const templateInvoiceApprovedEmailRaw = `
Your {{.Date}} invoice has just been approved! You will soon receive payment in DCR for the billed amount at the DCR/USD rate for {{.Date}}.

Invoice token: {{.Token}}
`

const templateInvoiceRejectedEmailRaw = `
Your {{.Date}} invoice has been rejected, please re-submit it with the requested revision(s).

Invoice token: {{.Token}}
Reason for rejection: {{.Reason}}
`

const templateInvoicePaidEmailRaw = `
You have received payment for your {{.Date}} invoice!

Invoice token: {{.Token}}
Transaction: {{.TxID}}
`
