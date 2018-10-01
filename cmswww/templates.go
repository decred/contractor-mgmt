// Copyright (c) 2018 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

const templateRegisterEmailRaw = `
<div>
	You are invited to join Decred as a contractor! To complete your registration,
	you will need to download <a href="#">this command-line program</a>
	 and execute it as follows:
</div>
<div style="margin: 20px 0 0 10px">
	<pre><code>
$ cmswwwcli register {{.Email}} &lt;username> &lt;password> {{.Token}}
	</code></pre>
</div>
<div style="margin-top: 20px">
	You are receiving this email because <span style="font-weight: bold">{{.Email}}</span>
	 was invited to join Decred. If you have no knowledge of this invitation,
	 please ignore this email.
</div>
`

const templateNewIdentityEmailRaw = `
<div>
	You have generated a new identity. To verify and start using it,
	you will need to execute the following:
</div>
<div style="margin: 20px 0 0 10px">
	<pre><code>
$ cmswwwcli login {{.Email}} &lt;password>
$ cmswwwcli verifyidentity {{.Token}}
	</code></pre>
</div>
<div style="margin-top: 20px">
	You are receiving this email because a new identity (public key:
	 {{.PublicKey}}) was generated for <span style="font-weight: bold">{{.Email}}</span>
	 on Decred Contractor Management. If you did not perform this action,
	 please notify the administrators.</div>
</div>
`

const templateUserLockedResetPasswordRaw = `
<div>
	Your account was locked due to too many login attempts. You need to
	reset your password in order to unlock your account by executing
	the following:
</div>
<div style="margin: 20px 0 0 10px">
	<pre><code>
$ cmswwwcli resetpassword {{.Email}}
	</code></pre>
</div>
<div style="margin-top: 20px">
	You are receiving this email because someone made too many login attempts
	 for <span style="font-weight: bold">{{.Email}}</span> on Decred Contractor
	 Management. If that was not you, please notify the administrators.
</div>
`

const templateResetPasswordEmailRaw = `
<div>
	You have reset your password. To verify your password reset,
	you will need to execute the following:
</div>
<div style="margin: 20px 0 0 10px">
	<pre><code>
$ cmswwwcli resetpassword {{.Email}} --token={{.Token}} --newpassword=&lt;your new password>
	</code></pre>
</div>
<div style="margin-top: 20px">
	You are receiving this email because the password has been reset for
	 <span style="font-weight: bold">{{.Email}}</span> on Decred Contractor
	 Management. If you did not perform this action, please notify the
	 administrators.</div>
</div>
`

const templateUpdateExtendedPublicKeyEmailRaw = `
<div>
	To update your extended public key, you will need to execute the following:
</div>
<div style="margin: 20px 0 0 10px">
	<pre><code>
$ cmswwwcli updatexpublickey --token={{.Token}} --xpublickey=&lt;your extended public key>
	</code></pre>
</div>
<div style="margin-top: 20px">
	You are receiving this email because it has been requested to update the
	 extended public key for <span style="font-weight: bold">{{.Email}}</span>
	 on Decred Contractor Management. If you did not perform this action,
	 please notify the administrators.</div>
</div>
`
