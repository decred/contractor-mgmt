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
$ cmswwwcli

  > newuser &lt;email> &lt;username> &lt;password> {{.Token}}
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
$ cmswwwcli

  > login &lt;email> &lt;password>
  > verifyidentity {{.Token}}
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
<div>Your account was locked due to too many login attempts. You need to reset your password in order to unlock your account:</div>
<div style="margin: 20px 0 0 10px"><a href="{{.Link}}">{{.Link}}</a></div>
<div style="margin-top: 20px">You are receiving this email because someone made too many login attempts for <span style="font-weight: bold">{{.Email}}</span> on Politeia.</div>
<div>If that was not you, please notify Politeia administrators.</div>
`
