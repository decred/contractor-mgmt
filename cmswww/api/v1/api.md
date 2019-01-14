# cmswww API Specification

# v1

This document describes the REST API provided by a `cmswww` server.  The
`cmswww` server is the web server backend and it interacts with a JSON REST
API.  It does not render HTML.

**Methods**

- [`Version`](#version)
- [`Invite new user`](#invite-new-user)
- [`Register`](#register)
- [`Login`](#login)
- [`Logout`](#logout)
- [`User details`](#user-details)
- [`Manage user`](#manage-user)
- [`Edit user`](#edit-user)
- [`Edit user extended pubkey`](#edit-user-extended-pubkey)
- [`New identity`](#new-identity)
- [`Verify new identity`](#verify-new-identity)
- [`Change password`](#change-password)
- [`Reset password`](#reset-password)
- [`Users`](#users)
- [`Invoices`](#invoices)
- [`User invoices`](#user-invoices)
- [`Review invoices`](#review-invoices)
- [`Pay invoices`](#pay-invoices)
- [`Update invoice payment`](#update-invoice-payment)
- [`Submit invoice`](#submit-invoice)
- [`Invoice details`](#invoice-details)
- [`Set invoice status`](#set-invoice-status)
- [`Policy`](#policy)

**Error status codes**

- [`ErrorStatusInvalid`](#ErrorStatusInvalid)
- [`ErrorStatusInvalidEmailOrPassword`](#ErrorStatusInvalidEmailOrPassword)
- [`ErrorStatusMalformedEmail`](#ErrorStatusMalformedEmail)
- [`ErrorStatusVerificationTokenInvalid`](#ErrorStatusVerificationTokenInvalid)
- [`ErrorStatusVerificationTokenExpired`](#ErrorStatusVerificationTokenExpired)
- [`ErrorStatusInvoiceMissingFiles`](#ErrorStatusInvoiceMissingFiles)
- [`ErrorStatusInvoiceNotFound`](#ErrorStatusInvoiceNotFound)
- [`ErrorStatusInvoiceDuplicateFilenames`](#ErrorStatusInvoiceDuplicateFilenames)
- [`ErrorStatusInvoiceInvalidTitle`](#ErrorStatusInvoiceInvalidTitle)
- [`ErrorStatusMaxMDsExceededPolicy`](#ErrorStatusMaxMDsExceededPolicy)
- [`ErrorStatusMaxImagesExceededPolicy`](#ErrorStatusMaxImagesExceededPolicy)
- [`ErrorStatusMaxMDSizeExceededPolicy`](#ErrorStatusMaxMDSizeExceededPolicy)
- [`ErrorStatusMaxImageSizeExceededPolicy`](#ErrorStatusMaxImageSizeExceededPolicy)
- [`ErrorStatusMalformedPassword`](#ErrorStatusMalformedPassword)
- [`ErrorStatusCommentNotFound`](#ErrorStatusCommentNotFound)
- [`ErrorStatusInvalidInvoiceName`](#ErrorStatusInvalidInvoiceName)
- [`ErrorStatusInvalidFileDigest`](#ErrorStatusInvalidFileDigest)
- [`ErrorStatusInvalidBase64`](#ErrorStatusInvalidBase64)
- [`ErrorStatusInvalidMIMEType`](#ErrorStatusInvalidMIMEType)
- [`ErrorStatusUnsupportedMIMEType`](#ErrorStatusUnsupportedMIMEType)
- [`ErrorStatusInvalidInvoiceStatusTransition`](#ErrorStatusInvalidInvoiceStatusTransition)
- [`ErrorStatusInvalidPublicKey`](#ErrorStatusInvalidPublicKey)
- [`ErrorStatusNoPublicKey`](#ErrorStatusNoPublicKey)
- [`ErrorStatusInvalidSignature`](#ErrorStatusInvalidSignature)
- [`ErrorStatusInvalidInput`](#ErrorStatusInvalidInput)
- [`ErrorStatusInvalidSigningKey`](#ErrorStatusInvalidSigningKey)
- [`ErrorStatusCommentLengthExceededPolicy`](#ErrorStatusCommentLengthExceededPolicy)
- [`ErrorStatusUserNotFound`](#ErrorStatusUserNotFound)
- [`ErrorStatusWrongStatus`](#ErrorStatusWrongStatus)
- [`ErrorStatusNotLoggedIn`](#ErrorStatusNotLoggedIn)
- [`ErrorStatusUserNotPaid`](#ErrorStatusUserNotPaid)
- [`ErrorStatusReviewerAdminEqualsAuthor`](#ErrorStatusReviewerAdminEqualsAuthor)
- [`ErrorStatusMalformedUsername`](#ErrorStatusMalformedUsername)
- [`ErrorStatusDuplicateUsername`](#ErrorStatusDuplicateUsername)
- [`ErrorStatusVerificationTokenUnexpired`](#ErrorStatusVerificationTokenUnexpired)
- [`ErrorStatusCannotVerifyPayment`](#ErrorStatusCannotVerifyPayment)
- [`ErrorStatusDuplicatePublicKey`](#ErrorStatusDuplicatePublicKey)
- [`ErrorStatusInvalidInvoiceVoteStatus`](#ErrorStatusInvalidInvoiceVoteStatus)
- [`ErrorStatusNoInvoiceCredits`](#ErrorStatusNoInvoiceCredits)
- [`ErrorStatusInvalidUserManageAction`](#ErrorStatusInvalidUserManageAction)

**Invoice status codes**

- [`InvoiceStatusInvalid`](#InvoiceStatusInvalid)
- [`InvoiceStatusNotFound`](#InvoiceStatusNotFound)
- [`InvoiceStatusNotReviewed`](#InvoiceStatusNotReviewed)
- [`InvoiceStatusUnreviewedChanges`](#InvoiceStatusUnreviewedChanges)
- [`InvoiceStatusRejected`](#InvoiceStatusRejected)
- [`InvoiceStatusApproved`](#InvoiceStatusApproved)
- [`InvoiceStatusPaid`](#InvoiceStatusPaid)

## HTTP status codes and errors

All methods, unless otherwise specified, shall return `200 OK` when successful,
`400 Bad Request` when an error has occurred due to user input, or `500 Internal Server Error`
when an unexpected server error has occurred. The format of errors is as follows:

**`4xx` errors**

| | Type | Description |
|-|-|-|
| errorcode | number | One of the [error codes](#error-codes) |
| errorcontext | Array of Strings | This array of strings is used to provide additional information for certain errors; see the documentation for specific error codes. |

**`5xx` errors**

| | Type | Description |
|-|-|-|
| errorcode | number | An error code that can be used to track down the internal server error that occurred; it should be reported to Politeia administrators. |

## Methods

### `Version`

Obtain version, route information and signing identity from server, as well as
the user info for the current session, if there is one.  This call shall
**ALWAYS** be the first contact with the server.  This is done in order to get
the CSRF token for the session and to ensure API compatability.

**Route**: `GET /`

**Params**: none

**Results**:

| | Type | Description |
|-|-|-|
| version | number | API version that is running on this server. |
| route | string | Route that should be prepended to all calls. For example, "/v1". |
| pubkey | string | The public key for the corresponding private key that signs various tokens to ensure server authenticity and to prevent replay attacks. |
| testnet | boolean | Value to inform either its running on testnet or not |
| user | [`Login reply`](#login-reply) | Information about the currently logged in user |

**Example**

Request:

```json
{}
```

Reply:

```json
{
  "version": 1,
  "route": "/v1",
  "identity": "99e748e13d7ecf70ef6b5afa376d692cd7cb4dbb3d26fa83f417d29e44c6bb6c",
  "testnet": true,
  "user": {
    "isadmin": false,
    "userid": "2238723947237",
    "email": "69af376cca42cd9c@example.com",
    "username": "foobar",
    "publickey": "5203ab0bb739f3fc267ad20c945b81bcb68ff22414510c000305f4f0afb90d1b",
    "lastlogin": 0
  }
}
```

### `Invite new user`

Create a new user on the cmswww server with a registration token and email
an invitation to them to register.

Note: This call requires admin privileges.

**Route:** `POST /v1/user/invite`

**Params:**

| Parameter | Type | Description | Required |
|-|-|-|-|
| email | string | Email is used as the web site user identity for a user. | Yes |

**Results:**

| Parameter | Type | Description |
|-|-|-|
| verificationtoken | String | The verification token which is required when calling [`Register`](#register). If an email server is set up, this property will be empty or nonexistent; the token will be sent to the email address sent in the request.|

This call can return one of the following error codes:

- [`ErrorStatusUserAlreadyExists`](#ErrorStatusUserAlreadyExists)
- [`ErrorStatusVerificationTokenUnexpired`](#ErrorStatusVerificationTokenUnexpired)

* **Example**

Request:

```json
{
  "email": "69af376cca42cd9c@example.com"
}
```

Reply:

```json
{
  "verificationtoken": "fc8f660e7f4d590e27e6b11639ceeaaec2ce9bc6b0303344555ac023ab8ee55f"
}
```

### `Register`

Verifies email address of a user account invited via
[`Invite new user`](#invite-new-user) and supply details for new user registration.

**Route:** `POST /v1/user/new`

**Params:**

| Parameter | Type | Description | Required |
|-|-|-|-|
| email | string | Email address of the user. | Yes |
| verificationtoken | string | The token that was provided by email to the user. | Yes |
| publickey | string | The user's ed25519 public key. | Yes |
| signature | string | The ed25519 signature of the string representation of the verification token. | Yes |
| username | string | A unique username for the user. | Yes |
| password | string | A password for the user. | Yes |
| name | string | The user's full name. | Yes |
| location | string | The user's physical location. | Yes |
| xpublickey | string | The extended public key for the user's payment account. | Yes |

**Results:** none

This call can return one of the following error codes:

- [`ErrorStatusVerificationTokenInvalid`](#ErrorStatusVerificationTokenInvalid)
- [`ErrorStatusVerificationTokenExpired`](#ErrorStatusVerificationTokenExpired)
- [`ErrorStatusInvalidPublicKey`](#ErrorStatusInvalidPublicKey)
- [`ErrorStatusMalformedUsername`](#ErrorStatusMalformedUsername)
- [`ErrorStatusDuplicateUsername`](#ErrorStatusDuplicateUsername)
- [`ErrorStatusMalformedPassword`](#ErrorStatusMalformedPassword)
- [`ErrorStatusDuplicatePublicKey`](#ErrorStatusDuplicatePublicKey)
- [`ErrorStatusInvalidSignature`](#ErrorStatusInvalidSignature

**Example:**

Request:

```json
{
  "email": "69af376cca42cd9c@example.com",
  "verificationtoken": "fc8f660e7f4d590e27e6b11639ceeaaec2ce9bc6b0303344555ac023ab8ee55f",
  "publickey": "5203ab0bb739f3fc267ad20c945b81bcb68ff22414510c000305f4f0afb90d1b",
  "signature":"9e4b1018913610c12496ec3e482f2fb42129197001c5d35d4f5848b77d2b5e5071f79b18bcab4f371c5b378280bb478c153b696003ac3a627c3d8a088cd5f00d",
  "username": "foobar",
  "password": "69af376cca42cd9c",
  "name": "John Smith",
  "location": "Atlanta, GA, USA",
  "xpublickey": "9e4b1018913610c12496ec3e482f2fb42129197001c5d35d4f5848b77d2b5e5071f79b18bcab4f371c5b378280bb478c153b696003ac3a627c3d8a088cd5f00d"
}
```

Reply:

```json
{}
```

### `Login`

Login as a user or admin.  Admin status is determined by the server based on
the user database.  Note that Login reply is identical to Me reply.

**Route:** `POST /v1/login`

**Params:**

| Parameter | Type | Description | Required |
|-|-|-|-|
| email | string | Email address of user that is attempting to login. | Yes |
| password | string | Accompanying password for provided email. | Yes |

**Results:** See the [`Login reply`](#login-reply).

On failure the call shall return `401 Unauthorized` and one of the following
error codes:
- [`ErrorStatusInvalidEmailOrPassword`](#ErrorStatusInvalidEmailOrPassword)
- [`ErrorStatusUserLocked`](#ErrorStatusUserLocked)

**Example**

Request:

```json
{
  "email":"69af376cca42cd9c@example.com",
  "password":"69af376cca42cd9c"
}
```

Reply:

```json
{
  "isadmin": false,
  "userid": "2238723947237",
  "email": "69af376cca42cd9c@example.com",
  "username": "foobar",
  "publickey": "5203ab0bb739f3fc267ad20c945b81bcb68ff22414510c000305f4f0afb90d1b",
  "lastlogin": 0
}
```

### `Logout`

Logout as a user or admin.

**Route:** `POST /v1/logout`

**Params:** none

**Results:** none

**Example**

Request:

```json
{}
```

Reply:

```json
{}
```

### `User details`

Returns details about a user given either its id, email or username.

Note: This call requires admin privileges.

**Route:** `GET /v1/user`

**Params:**

| Parameter | Type | Description | Required |
|-----------|------|-------------|----------|
| userid | string | The unique id of the user. | Yes |
| email | string | The user's email address. | Yes |
| username | string | The unique username of the user. | Yes |

**Results:**

| Parameter | Type | Description |
|-|-|-|
| user | [User](#user) | The user details. |

This call can return one of the following error codes:

- [`ErrorStatusUserNotFound`](#ErrorStatusUserNotFound)

**Example**

Request:

```json
{
  "userid": "0"
}
```

Reply:

```json
{
  "user": {
    "id": "0",
    "email": "6b87b6ebb0c80cb7@example.com",
    "username": "6b87b6ebb0c80cb7",
    "isadmin": false,
    "newuserpaywalladdress": "Tsgs7qb1Gnc43D9EY3xx9ou8Lbo8rB7me6M",
    "newuserpaywallamount": 10000000,
    "newuserpaywalltxnotbefore": 1528821554,
    "newuserpaywalltx": "",
    "newuserpaywallpollexpiry": 1528821554,
    "newuserverificationtoken": "337fc4762dac6bbe11d3d0130f33a09978004b190e6ebbbde9312ac63f223527",
    "newuserverificationexpiry": 1528821554,
    "updatekeyverificationtoken": "337fc4762dac6bbe11d3d0130f33a09978004b190e6ebbbde9312ac63f223527",
    "updatekeyverificationexpiry": 1528821554,
    "resetpasswordverificationtoken": "337fc4762dac6bbe11d3d0130f33a09978004b190e6ebbbde9312ac63f223527",
    "resetpasswordverificationexpiry": 1528821554,
    "identities": [{
      "pubkey": "5203ab0bb739f3fc267ad20c945b81bcb68ff22414510c000305f4f0afb90d1b",
      "isactive": true
    }],
    "invoices": []
  }
}
```

### `Manage user`

Performs a specific action on a user given their id, email or username.

Note: This call requires admin privileges.

**Route:** `POST /v1/user/manage`

**Params:**

| Parameter | Type | Description | Required |
|-----------|------|-------------|----------|
| userid | string | The unique id of the user. | Yes |
| email | string | The user's email address. | Yes |
| username | string | The unique username of the user. | Yes |
| action | int64 | The [user manage action](#user-manage-actions) to execute on the user. | Yes |
| reason | string | The admin's reason for executing this action. | Yes |

**Results:**

| Parameter | Type | Description |
|-|-|-|
| verificationtoken | string | The verification token created; only set for the [`UserManageResendInvite`](#UserManageResendInvite) action. |

This call can return one of the following error codes:

- [`ErrorStatusUserNotFound`](#ErrorStatusUserNotFound)
- [`ErrorStatusInvalidInput`](#ErrorStatusInvalidInput)
- [`ErrorStatusInvalidUserManageAction`](#ErrorStatusInvalidUserManageAction)

**Example**

Request:

```json
{
  "userid": "0",
  "action": 1
}
```

Reply:

```json
{}
```

### `Edit user`

Edits a user's preferences.

**Route:** `POST /v1/user/edit`

**Params:**

| Parameter | Type | Description | Required |
|-|-|-|-|
| name | string | The user's new name. | |
| location | string | The user's new physical location. | |
| emailnotifications | int64 | The total of the values that correspond to the [`email notification types`](#email-notification-types) which the user is opting into. | |

**Results:** none

**Example**

Request:

```json
{
  "name": "Jonathan Smith",
  "emailnotifications": 0
}
```

Reply:

```json
{}
```

### `Edit user extended pubkey`

Edits a user's extended public key.

**Route:** `POST /v1/user/edit/xpublickey`

**Params:**

| Parameter | Type | Description | Required |
|-|-|-|-|
| xpublickey | string | The user's new extended public key. | Yes, on the follow-up call |
| verificationtoken | string | The user's new physical location. | Yes, on the follow-up call |

This command is special.  It must be called **twice** with different
parameters.

For the 1st call, it should not be called with any parameters. On success it will
send an email to the user's email address containing a verification token and
return `200 OK`. For the 2nd call, it should be called with an `xpublickey`
parameter, as well as the verification token provided in the email. On success
it will change the user's extended public key and return `200 OK`.

This call can return one of the following error codes:

- [`ErrorStatusVerificationTokenInvalid`](#ErrorStatusVerificationTokenInvalid)
- [`ErrorStatusVerificationTokenExpired`](#ErrorStatusVerificationTokenExpired)

**Results:**

| Parameter | Type | Description |
|-|-|-|
| verificationtoken | string | The verification token which is required when making the follow-up call. If an email server is set up, this property will be empty or nonexistent; the token will be sent to the user's email address. |

**Example (2nd call)**

Request:

```json
{
  "xpublickey": "9e4b1018913610c12496ec3e482f2fb42129197001c5d35d4f5848b77d2b5e5071f79b18bcab4f371c5b378280bb478c153b696003ac3a627c3d8a088cd5f00d",
  "verificationtoken": "fc8f660e7f4d590e27e6b11639ceeaaec2ce9bc6b0303344555ac023ab8ee55f"
}
```

Reply:

```json
{}
```

### `New identity`

Sets a new active key pair for a user.

**Route:** `POST /v1/user/identity`

**Params:**

| Parameter | Type | Description | Required |
|-|-|-|-|
| publickey | string | User's new active ed25519 public key. | Yes |

**Results:**

| Parameter | Type | Description |
|-|-|-|
| verificationtoken | String | The verification token which is required when calling [`Verify new identity`](#verify-new-identity). If an email server is set up, this property will be empty or nonexistent; the token will be sent to the email address sent in the request. |

This call can return one of the following error codes:

- [`ErrorStatusInvalidPublicKey`](#ErrorStatusInvalidPublicKey)
- [`ErrorStatusDuplicatePublicKey`](#ErrorStatusDuplicatePublicKey)
- [`ErrorStatusVerificationTokenUnexpired`](#ErrorStatusVerificationTokenUnexpired)

* **Example**

Request:

```json
{
  "publickey":"5203ab0bb739f3fc267ad20c945b81bcb68ff22414510c000305f4f0afb90d1b"
}
```

Reply:

```json
{
  "verificationtoken": "fc8f660e7f4d590e27e6b11639ceeaaec2ce9bc6b0303344555ac023ab8ee55f"
}
```

### `Verify new identity`

Verify the new key pair for the user.

**Route:** `POST /v1/user/identity/verify`

**Params:**

| Parameter | Type | Description | Required |
|-|-|-|-|
| verificationtoken | string | The token that was provided by email to the user. | Yes |
| signature | string | The ed25519 signature of the string representation of the verification token. | Yes |

**Results:** none

This call can return one of the following error codes:

- [`ErrorStatusVerificationTokenInvalid`](#ErrorStatusVerificationTokenInvalid)
- [`ErrorStatusVerificationTokenExpired`](#ErrorStatusVerificationTokenExpired)
- [`ErrorStatusInvalidSignature`](#ErrorStatusInvalidSignature)

**Example:**

Request:

The request params should be provided within the URL:

```json
{
  "verificationtoken":"f1c2042d36c8603517cf24768b6475e18745943e4c6a20bc0001f52a2a6f9bde",
  "signature":"9e4b1018913610c12496ec3e482f2fb42129197001c5d35d4f5848b77d2b5e5071f79b18bcab4f371c5b378280bb478c153b696003ac3a627c3d8a088cd5f00d"
}
```

Reply:

```json
{}
```

### `Change password`

Changes the password for the currently logged in user.

**Route:** `POST /v1/user/password/change`

**Params:**

| Parameter | Type | Description | Required |
|-|-|-|-|
| currentpassword | string | The current password of the logged in user. | Yes |
| newpassword | string | The new password for the logged in user. | Yes |

**Results:** none

On failure the call shall return `400 Bad Request` and one of the following
error codes:
- [`ErrorStatusInvalidEmailOrPassword`](#ErrorStatusInvalidEmailOrPassword)
- [`ErrorStatusMalformedPassword`](#ErrorStatusMalformedPassword)

**Example**

Request:

```json
{
  "currentpassword": "15a1eb6de3681fec",
  "newpassword": "cef1863ed6be1a51"
}
```

Reply:

```json
{}
```

### `Reset password`

Allows a user to reset his password without being logged in.

**Route:** `POST /v1/user/password/reset`

**Params:**

| Parameter | Type | Description | Required |
|-|-|-|-|
| email | string | The email of the user whose password should be reset. | Yes |
| verificationtoken | string | The verification token which is sent to the user's email address. | Yes |
| newpassword | String | The new password for the user. | Yes |

**Results:**

| Parameter | Type | Description |
|-|-|-|
| verificationtoken | String | This command is special because it has to be called twice, the 2nd time the caller needs to supply the `verificationtoken` |


The reset password command is special.  It must be called **twice** with different
parameters.

For the 1st call, it should be called with only an `email` parameter. On success
it shall send an email to the address provided by `email` and return `200 OK`.

The email shall include a link in the following format:

```
/v1/user/password/reset?email=abc@example.com&verificationtoken=f1c2042d36c8603517cf24768b6475e18745943e4c6a20bc0001f52a2a6f9bde
```

On failure, the call shall return `400 Bad Request` and one of the following
error codes:
- [`ErrorStatusMalformedEmail`](#ErrorStatusMalformedEmail)

For the 2nd call, it should be called with `email`, `token`, and `newpassword`
parameters.

On failure, the call shall return `400 Bad Request` and one of the following
error codes:
- [`ErrorStatusVerificationTokenInvalid`](#ErrorStatusVerificationTokenInvalid)
- [`ErrorStatusVerificationTokenExpired`](#ErrorStatusVerificationTokenExpired)
- [`ErrorStatusMalformedPassword`](#ErrorStatusMalformedPassword)

**Example for the 1st call**

Request:

```json
{
  "email": "6b87b6ebb0c80cb7@example.com"
}
```

Reply:

```json
{}
```

**Example for the 2nd call**

Request:

```json
{
  "email": "6b87b6ebb0c80cb7@example.com",
  "verificationtoken": "f1c2042d36c8603517cf24768b6475e18745943e4c6a20bc0001f52a2a6f9bde",
  "newpassword": "6b87b6ebb0c80cb7"
}
```

Reply:

```json
{
  "verificationtoken": "f1c2042d36c8603517cf24768b6475e18745943e4c6a20bc0001f52a2a6f9bde"
}
```

### `Users`

Returns a list of users, optionally filtered by username.

Note: This call requires admin privileges.

**Route:** `GET /v1/users`

**Params:**

| Parameter | Type | Description | Required |
|-|-|-|-|
| username | string | Optional filter for the list of users to match (or partially match) the usernames. | |
| page | uint16 | The page number of users to fetch (starts at 0). | |

**Results:**

| Parameter | Type | Description |
|-|-|-|
| users | array of [`Abridged user`](#abridged-user)s | The list of users, capped by the `listpagesize` (see the `Policy`(#policy) call for details) |
| totalmatches | int64 | The total number of users matched |

**Example**

Request:

```json
{
  "username": "foo"
}
```

Reply:

```json
{
  "users": [{
    "id": "0",
    "email": "foobar@example.com",
    "username": "foobar",
    "isadmin": false
  }],
  "totalmatches": 1
}
```

### `Invoices`

Retrieve a page of invoices given the month and year; the number of invoices
returned in the page is limited by the `listpagesize` property, which is
provided via [`Policy`](#policy).

Note: This call requires admin privileges.

**Route:** `GET /v1/invoices`

**Params:**

| Parameter | Type | Description | Required |
|-|-|-|-|
| month | int16 | A specific month, from 1 to 12. | Yes |
| year | int16 | A specific year. | Yes |
| status | int64 | An optional filter for the list; this should be an [invoice status](#invoice-status-codes). | |
| page | uint16 | The page number of invoices to fetch (starts at 0). | |

**Results:**

| | Type | Description |
|-|-|-|
| invoices | array of [`Invoice`](#invoice)s | The page of invoices. |
| totalmatches | int64 | The total number of invoices matched. |

**Example**

Request:

```json
{
  "month": 12,
  "year": 2018
}
```

Reply:

```json
{
  "invoices": [{
    "status": 4,
    "month": 12,
    "year": 2018,
    "timestamp": 1508296860781,
    "userid": "0",
    "username": "foobar",
    "publickey":"5203ab0bb739f3fc267ad20c945b81bcb68ff22414510c000305f4f0afb90d1b",
    "signature": "gdd92f26c8g38c90d2887259e88df614654g32fde76bef1438b0efg40e360f461e995d796g16b17108gbe226793ge4g52gg013428feb3c39de504fe5g1811e0e",
    "version": "1",
    "censorshiprecord": {
      "token": "337fc4762dac6bbe11d3d0130f33a09978004b190e6ebbbde9312ac63f223527",
      "merkle": "0dd10219cd79342198085cbe6f737bd54efe119b24c84cbc053023ed6b7da4c8",
      "signature": "fcc92e26b8f38b90c2887259d88ce614654f32ecd76ade1438a0def40d360e461d995c796f16a17108fad226793fd4f52ff013428eda3b39cd504ed5f1811d0d"
    }
  }]
}
```

### `User invoices`

Returns a page of the user's invoices.

**Route:** `GET /v1/user/invoices`

**Params:**

| Parameter | Type | Description | Required |
|-|-|-|-|
| status | int64 | An optional filter for the list; this should be an [invoice status](#invoice-status-codes). | |
| page | uint16 | The page number of invoices to fetch (starts at 0). | |

**Results:**

| | Type | Description |
|-|-|-|
| invoices | array of [`Invoice`](#invoice)s | The page of invoices. |
| totalmatches | int64 | The total number of invoices matched. |

**Example**

Request:

```json
{
  "status": 4
}
```

Reply:

```json
{
  "invoices": [{
    "status": 4,
    "month": 12,
    "year": 2018,
    "timestamp": 1508296860781,
    "userid": "0",
    "username": "foobar",
    "publickey":"5203ab0bb739f3fc267ad20c945b81bcb68ff22414510c000305f4f0afb90d1b",
    "signature": "gdd92f26c8g38c90d2887259e88df614654g32fde76bef1438b0efg40e360f461e995d796g16b17108gbe226793ge4g52gg013428feb3c39de504fe5g1811e0e",
    "version": "1",
    "censorshiprecord": {
      "token": "337fc4762dac6bbe11d3d0130f33a09978004b190e6ebbbde9312ac63f223527",
      "merkle": "0dd10219cd79342198085cbe6f737bd54efe119b24c84cbc053023ed6b7da4c8",
      "signature": "fcc92e26b8f38b90c2887259d88ce614654f32ecd76ade1438a0def40d360e461d995c796f16a17108fad226793fd4f52ff013428eda3b39cd504ed5f1811d0d"
    }
  }]
}
```

### `Review invoices`

Retrieve all unreviewed invoices given the month and year.

Note: This call requires admin privileges.

**Route:** `POST /v1/invoices/review`

**Params:**

| Parameter | Type | Description | Required |
|-|-|-|-|
| month | int16 | A specific month, from 1 to 12. | Yes |
| year | int16 | A specific year. | Yes |

**Results:**

| | Type | Description |
|-|-|-|
| invoices | array of [`Invoice review`](#invoice-review)s | The array of all invoices to review. |

**Example**

Request:

```json
{
  "month": 12,
  "year": 2018
}
```

Reply:

```json
{
  "invoices": [{
    "token": "337fc4762dac6bbe11d3d0130f33a09978004b190e6ebbbde9312ac63f223527",
    "userid": "0",
    "username": "foobar",
    "totalhours": 10,
    "totalcostusd": 400,
    "lineitems": [{
      "type": "Development",
      "subtype": "",
      "description": "decred/politeia PR #34",
      "proposal": "",
      "hours": 5,
      "totalcost": 200
    }, {
      "type": "Development",
      "subtype": "",
      "description": "decred/politeia PR #38",
      "proposal": "",
      "hours": 5,
      "totalcost": 200
    }]
  }]
}
```

### `Pay invoices`

Retrieve all approved invoices given the month and year which are ready to be paid.

Note: This call requires admin privileges.

**Route:** `POST /v1/invoices/pay`

**Params:**

| Parameter | Type | Description | Required |
|-|-|-|-|
| month | int16 | A specific month, from 1 to 12. | Yes |
| year | int16 | A specific year. | Yes |
| dcrusdrate | float64 | The USD/DCR rate to use for generating invoice payment summaries. | Yes |

**Results:**

| | Type | Description |
|-|-|-|
| invoices | array of [`Invoice payment`](#invoice-payment)s | The array of all invoices to pay. |

**Example**

Request:

```json
{
  "month": 12,
  "year": 2018,
  "dcrusdrate": 20
}
```

Reply:

```json
{
  "invoices": [{
    "token": "337fc4762dac6bbe11d3d0130f33a09978004b190e6ebbbde9312ac63f223527",
    "userid": "0",
    "username": "foobar",
    "totalhours": 10,
    "totalcostusd": 400,
    "lineitems": [{
      "type": "Development",
      "subtype": "",
      "description": "decred/politeia PR #34",
      "proposal": "",
      "hours": 5,
      "totalcost": 200
    }, {
      "type": "Development",
      "subtype": "",
      "description": "decred/politeia PR #38",
      "proposal": "",
      "hours": 5,
      "totalcost": 200
    }]
  }]
}
```

### `Update invoice payment`

Updates an invoice payment with a Decred transaction id.

Note: This call requires admin privileges.

**Route:** `POST /v1/invoice/payments/update`

**Params:**

| Parameter | Type | Description | Required |
|-|-|-|-|
| token | string | The invoice token. | Yes |
| address | string | The payment address for the invoice. | Yes |
| amount | int64 | The USD/DCR rate to use for generating invoice payment summaries. | Yes |
| txid | string | The Decred transaction id for the payment. | Yes |

**Results:** none

**Example**

Request:

```json
{
  "token": "337fc4762dac6bbe11d3d0130f33a09978004b190e6ebbbde9312ac63f223527",
  "address": "TsWJuYPXZqczwkckGZnHUqXgi7FemNks48W",
  "amount": 5000000000,
  "txid": "9ab1f8413bb895f46088e317d42a950e929ab8649961cc4a9311cba5c7bff73a"
}
```

Reply:

```json
{}
```

### `Submit invoice`

Submit an invoice for the given month and year.

**Route:** `POST /v1/invoice/submit`

**Params:**

| Parameter | Type | Description | Required |
|-|-|-|-|
| month | int16 | A specific month, from 1 to 12. | Yes |
| year | int16 | A specific year. | Yes |
| file | [`File`](#file) | The invoice CSV file. The first line should be a comment with the month and year, with the format: `# 2006-01` | Yes |
| publickey | string | The user's public key. | Yes |
| signature | string | The signature of the string representation of the file payload. | Yes |

**Results:**

| Parameter | Type | Description |
|-|-|-|
| censorshiprecord | [CensorshipRecord](#censorship-record) | A censorship record that provides the submitter with a method to extract the invoice and prove that he/she submitted it. |

This call can return one of the following error codes:

- [`ErrorStatusInvalidSignature`](#ErrorStatusInvalidSignature)
- [`ErrorStatusInvalidSigningKey`](#ErrorStatusInvalidSigningKey)
- [`ErrorStatusNoPublicKey`](#ErrorStatusNoPublicKey)
- [`ErrorStatusInvalidInput`](#ErrorStatusInvalidInput)
- [`ErrorStatusMalformedInvoiceFile`](#ErrorStatusMalformedInvoiceFile)

**Example**

Request:

```json
{
  "month": 12,
  "year": 2018,
  "file": [{
      "digest": "0dd10219cd79342198085cbe6f737bd54efe119b24c84cbc053023ed6b7da4c8",
      "payload": "VGhpcyBpcyBhIGRlc2NyaXB0aW9u"
    }
  ]
}
```

Reply:

```json
{
  "censorshiprecord": {
    "token": "337fc4762dac6bbe11d3d0130f33a09978004b190e6ebbbde9312ac63f223527",
    "merkle": "0dd10219cd79342198085cbe6f737bd54efe119b24c84cbc053023ed6b7da4c8",
    "signature": "fcc92e26b8f38b90c2887259d88ce614654f32ecd76ade1438a0def40d360e461d995c796f16a17108fad226793fd4f52ff013428eda3b39cd504ed5f1811d0d"
  }
}
```

### `Policy`

Retrieve server policy.  The returned values contain various maxima that the client
SHALL observe.

**Route:** `GET /v1/policy`

**Params:** none

**Results:**

| | Type | Description |
|-|-|-|
| minpasswordlength | integer | minimum number of characters accepted for user passwords |
| minusernamelength | integer | minimum number of characters accepted for username |
| maxusernamelength | integer | maximum number of characters accepted for username |
| usernamesupportedchars | array of strings | the regular expression of a valid username |
| listpagesize | integer | maximum number of items returned for the routes that return lists |
| validmimetypes | array of strings | list of all acceptable MIME types that can be communicated between client and server. |
| invoice | [`Invoice policy`](#invoice-policy) | policy items specific to invoices |


**Example**

Request:

```
/v1/policy
```

Reply:

```json
{
  "minpasswordlength": 8,
  "minusernamelength": 3,
  "maxusernamelength": 30,
  "usernamesupportedchars": [
    "A-z", "0-9", ".", ":", ";", ",", "-", " ", "@", "+"
  ],
  "listpagesize": 20,
  "validmimetypes": [
    "text/plain; charset=utf-8"
  ],
  "invoice": {
    "fielddelimiterchar": ",",
    "commentchar": "#",
    "fields": [{
      "name": "Type of work",
      "type": 1,
      "required": true
    }]
  }
}
```

### `Set invoice status`

Sets the invoice status to either `InvoiceStatusApproved` or `InvoiceStatusRejected`.

Note: This call requires admin privileges.

**Route:** `POST /v1/invoice/status`

**Params:**

| Parameter | Type | Description | Required |
|-|-|-|-|
| token | string | Token is the unique censorship token that identifies a specific invoice. | Yes |
| status | number | The new [status](#invoice-status-codes) for the invoice. | Yes |
| reason | string | The reason for the new status. This is only required if the status is `InvoiceStatusRejected`. | |
| signature | string | Signature of token+string(status). | Yes |
| publickey | string | The user's public key, sent for signature verification. | Yes |

**Results:**

| Parameter | Type | Description |
|-|-|-|-|
| invoice | [`Invoice`](#invoice) | The updated invoice. |

This call can return one of the following error codes:

- [`ErrorStatusInvoiceNotFound`](#ErrorStatusInvoiceNotFound)

**Example**

Request:

```json
{
  "invoicestatus": 4,
  "publickey": "f5519b6fdee08be45d47d5dd794e81303688a8798012d8983ba3f15af70a747c",
  "signature": "041a12e5df95ec132be27f0c716fd8f7fc23889d05f66a26ef64326bd5d4e8c2bfed660235856da219237d185fb38c6be99125d834c57030428c6b96a2576900",
  "token": "337fc4762dac6bbe11d3d0130f33a09978004b190e6ebbbde9312ac63f223527"
}
```

Reply:

```json
{
  "invoice": {
    "status": 4,
    "month": 12,
    "year": 2018,
    "timestamp": 1508296860781,
    "userid": "0",
    "username": "foobar",
    "publickey":"5203ab0bb739f3fc267ad20c945b81bcb68ff22414510c000305f4f0afb90d1b",
    "signature": "gdd92f26c8g38c90d2887259e88df614654g32fde76bef1438b0efg40e360f461e995d796g16b17108gbe226793ge4g52gg013428feb3c39de504fe5g1811e0e",
    "version": "1",
    "censorshiprecord": {
      "token": "337fc4762dac6bbe11d3d0130f33a09978004b190e6ebbbde9312ac63f223527",
      "merkle": "0dd10219cd79342198085cbe6f737bd54efe119b24c84cbc053023ed6b7da4c8",
      "signature": "fcc92e26b8f38b90c2887259d88ce614654f32ecd76ade1438a0def40d360e461d995c796f16a17108fad226793fd4f52ff013428eda3b39cd504ed5f1811d0d"
    }
  }
}
```

### `Invoice details`

Retrieve an invoice's details.

**Routes:** `GET /v1/invoice`

**Params:**

| Parameter | Type | Description | Required |
|-|-|-|-|
| token | string | Token is the unique censorship token that identifies a specific invoice. | Yes |

**Results:**

| | Type | Description |
|-|-|-|
| invoice | [`Invoice`](#invoice) | The invoice with the provided token. |

This call can return one of the following error codes:

- [`ErrorStatusInvoiceNotFound`](#ErrorStatusInvoiceNotFound)

**Example**

Request:

```json
{
  "token": "337fc4762dac6bbe11d3d0130f33a09978004b190e6ebbbde9312ac63f223527"
}
```

Reply:

```json
{
  "invoice": {
    "status": 4,
    "month": 12,
    "year": 2018,
    "timestamp": 1508296860781,
    "userid": "0",
    "username": "foobar",
    "publickey":"5203ab0bb739f3fc267ad20c945b81bcb68ff22414510c000305f4f0afb90d1b",
    "signature": "gdd92f26c8g38c90d2887259e88df614654g32fde76bef1438b0efg40e360f461e995d796g16b17108gbe226793ge4g52gg013428feb3c39de504fe5g1811e0e",
    "version": "1",
    "censorshiprecord": {
      "token": "337fc4762dac6bbe11d3d0130f33a09978004b190e6ebbbde9312ac63f223527",
      "merkle": "0dd10219cd79342198085cbe6f737bd54efe119b24c84cbc053023ed6b7da4c8",
      "signature": "fcc92e26b8f38b90c2887259d88ce614654f32ecd76ade1438a0def40d360e461d995c796f16a17108fad226793fd4f52ff013428eda3b39cd504ed5f1811d0d"
    }
  }
}
```

### Error codes

| Status | Value | Description |
|-|-|-|
| <a name="ErrorStatusInvalid">ErrorStatusInvalid</a> | 0 | The operation returned an invalid status. This shall be considered a bug. |
| <a name="ErrorStatusInvalidEmailOrPassword">ErrorStatusInvalidEmailOrPassword</a> | 1 | Either the user name or password was invalid. |
| <a name="ErrorStatusMalformedEmail">ErrorStatusMalformedEmail</a> | 2 | The provided email address was malformed. |
| <a name="ErrorStatusVerificationTokenInvalid">ErrorStatusVerificationTokenInvalid</a> | 3 | The provided user activation token is invalid. |
| <a name="ErrorStatusVerificationTokenExpired">ErrorStatusVerificationTokenExpired</a> | 4 | The provided user activation token is expired. |
| <a name="ErrorStatusInvoiceMissingFiles">ErrorStatusInvoiceMissingFiles</a> | 5 | The provided invoice does not have files. This error may include additional context: index file is missing - "index.md". |
| <a name="ErrorStatusInvoiceNotFound">ErrorStatusInvoiceNotFound</a> | 6 | The requested invoice does not exist. |
| <a name="ErrorStatusInvoiceDuplicateFilenames">ErrorStatusInvoiceDuplicateFilenames</a> | 7 | The provided invoice has duplicate files. This error is provided with additional context: the duplicate name(s). |
| <a name="ErrorStatusInvoiceInvalidTitle">ErrorStatusInvoiceInvalidTitle</a> | 8 | The provided invoice title is invalid. This error is provided with additional context: the regular expression accepted. |
| <a name="ErrorStatusMaxMDsExceededPolicy">ErrorStatusMaxMDsExceededPolicy</a> | 9 | The submitted invoice has too many markdown files. Limits can be obtained by issuing the [Policy](#policy) command. |
| <a name="ErrorStatusMaxImagesExceededPolicy">ErrorStatusMaxImagesExceededPolicy</a> | 10 | The submitted invoice has too many images. Limits can be obtained by issuing the [Policy](#policy) command. |
| <a name="ErrorStatusMaxMDSizeExceededPolicy">ErrorStatusMaxMDSizeExceededPolicy</a> | 11 | The submitted invoice markdown is too large. Limits can be obtained by issuing the [Policy](#policy) command. |
| <a name="ErrorStatusMaxImageSizeExceededPolicy">ErrorStatusMaxImageSizeExceededPolicy</a> | 12 | The submitted invoice has one or more images that are too large. Limits can be obtained by issuing the [Policy](#policy) command. |
| <a name="ErrorStatusMalformedPassword">ErrorStatusMalformedPassword</a> | 13 | The provided password was malformed. |
| <a name="ErrorStatusCommentNotFound">ErrorStatusCommentNotFound</a> | 14 | The requested comment does not exist. |
| <a name="ErrorStatusInvalidInvoiceName">ErrorStatusInvalidInvoiceName</a> | 15 | The invoice's name was invalid. |
| <a name="ErrorStatusInvalidFileDigest">ErrorStatusInvalidFileDigest</a> | 16 | The digest (SHA-256 checksum) provided for one of the invoice files was incorrect. This error is provided with additional context: The name of the file with the invalid digest. |
| <a name="ErrorStatusInvalidBase64">ErrorStatusInvalidBase64</a> | 17 | The name of the file with the invalid encoding.The Base64 encoding provided for one of the invoice files was incorrect. This error is provided with additional context: the name of the file with the invalid encoding. |
| <a name="ErrorStatusInvalidMIMEType">ErrorStatusInvalidMIMEType</a> | 18 | The MIME type provided for one of the invoice files was not the same as the one derived from the file's content. This error is provided with additional context: The name of the file with the invalid MIME type and the MIME type detected for the file's content. |
| <a name="ErrorStatusUnsupportedMIMEType">ErrorStatusUnsupportedMIMEType</a> | 19 | The MIME type provided for one of the invoice files is not supported. This error is provided with additional context: The name of the file with the unsupported MIME type and the MIME type that is unsupported. |
| <a name="ErrorStatusInvalidInvoiceStatusTransition">ErrorStatusInvalidInvoiceStatusTransition</a> | 20 | The provided invoice cannot be changed to the given status. |
| <a name="ErrorStatusInvalidPublicKey">ErrorStatusInvalidPublicKey</a> | 21 | Invalid public key. |
| <a name="ErrorStatusNoPublicKey">ErrorStatusNoPublicKey</a> | 22 | User does not have an active public key. |
| <a name="ErrorStatusInvalidSignature">ErrorStatusInvalidSignature</a> | 23 | Invalid signature. |
| <a name="ErrorStatusInvalidInput">ErrorStatusInvalidInput</a> | 24 | Invalid input. |
| <a name="ErrorStatusInvalidSigningKey">ErrorStatusInvalidSigningKey</a> | 25 | Invalid signing key. |
| <a name="ErrorStatusCommentLengthExceededPolicy">ErrorStatusCommentLengthExceededPolicy</a> | 26 | The submitted comment length is too large. |
| <a name="ErrorStatusUserNotFound">ErrorStatusUserNotFound</a> | 27 | The user was not found. |
| <a name="ErrorStatusWrongStatus">ErrorStatusWrongStatus</a> | 28 | The invoice has the wrong status. |
| <a name="ErrorStatusNotLoggedIn">ErrorStatusNotLoggedIn</a> | 29 | The user must be logged in for this action. |
| <a name="ErrorStatusUserNotPaid">ErrorStatusUserNotPaid</a> | 30 | The user hasn't paid the registration fee. |
| <a name="ErrorStatusReviewerAdminEqualsAuthor">ErrorStatusReviewerAdminEqualsAuthor</a> | 31 | The user cannot change the status of his own invoice. |
| <a name="ErrorStatusMalformedUsername">ErrorStatusMalformedUsername</a> | 32 | The provided username was malformed. |
| <a name="ErrorStatusDuplicateUsername">ErrorStatusDuplicateUsername</a> | 33 | The provided username is already taken by another user. |
| <a name="ErrorStatusVerificationTokenUnexpired">ErrorStatusVerificationTokenUnexpired</a> | 34 | A verification token has already been generated and hasn't expired yet. |
| <a name="ErrorStatusCannotVerifyPayment">ErrorStatusCannotVerifyPayment</a> | 35 | The server cannot verify the payment at this time, please try again later. |
| <a name="ErrorStatusDuplicatePublicKey">ErrorStatusDuplicatePublicKey</a> | 36 | The public key provided is already taken by another user. |
| <a name="ErrorStatusInvalidInvoiceVoteStatus">ErrorStatusInvalidInvoiceVoteStatus</a> | 37 | Invalid invoice vote status. |
| <a name="ErrorStatusUserLocked">ErrorStatusUserLocked</a> | 38 | User locked due to too many login attempts. |
| <a name="ErrorStatusNoInvoiceCredits">ErrorStatusNoInvoiceCredits</a> | 39 | No invoice credits. |
| <a name="ErrorStatusInvalidUserManageAction">ErrorStatusInvalidUserManageAction</a> | 40 | Invalid action for editing a user. |

### Invoice status codes

| Status | Value | Description |
|-|-|-|
| <a name="InvoiceStatusInvalid">InvoiceStatusInvalid</a>| 0 | An invalid status. This shall be considered a bug. |
| <a name="InvoiceStatusNotFound">InvoiceStatusNotFound</a> | 1 | The invoice was not found. |
| <a name="InvoiceStatusNotReviewed">InvoiceStatusNotReviewed</a> | 2 | The invoice has not been reviewed by an admin. |
| <a name="InvoiceStatusUnreviewedChanges">InvoiceStatusUnreviewedChanges</a> | 3 | The invoice has been changed and the changes have not been reviewed by an admin. |
| <a name="InvoiceStatusRejected">InvoiceStatusRejected</a> | 4 | The invoice has been rejected by an admin. |
| <a name="InvoiceStatusApproved">InvoiceStatusApproved</a> | 5 | The invoice has been approved by an admin. |
| <a name="InvoiceStatusPaid">InvoiceStatusPaid</a> | 6 | The invoice has been paid. |

### User manage actions

| Status | Value | Description |
|-|-|-|
| <a name="UserManageInvalid">UserManageInvalid</a>| 0 | An invalid action. This shall be considered a bug. |
| <a name="UserManageResendInvite">UserManageResendInvite</a> | 1 | Resends the invitation email. |
| <a name="UserManageExpireUpdateIdentityVerification">UserManageExpireUpdateIdentityVerification</a> | 2 | Resends the update identity verification email. |
| <a name="UserManageUnlock">UserManageUnlock</a> | 3 | Unlocks a user's account. |
| <a name="UserManageLock">UserManageLock</a> | 4 | Locks a user's account. |

### `User`

| | Type | Description |
|-|-|-|
| id | string | The unique id of the user. |
| email | string | Email address. |
| username | string | Unique username. |
| isadmin | boolean | Whether the user is an admin or not. |
| location | string | User's physical location. |
| xpublickey | string | The extended public key for the user's payment account. |
| newuserverificationtoken | string | The verification token which is sent to the user's email address after inviting. |
| newuserverificationexpiry | int64 | The UNIX time (in seconds) for when the `newuserverificationtoken` expires. |
| updatekeyverificationtoken | string | The verification token which is sent to the user's email address for creating a new identity. |
| updatekeyverificationexpiry | int64 | The UNIX time (in seconds) for when the `updatekeyverificationtoken` expires. |
| resetpasswordverificationtoken | string | The verification token which is sent to the user's email address for resetting his password. |
| resetpasswordverificationexpiry | int64 | The UNIX time (in seconds) for when the `resetpasswordverificationtoken` expires. |
| updatexpublickeyverificationtoken | string | The verification token which is sent to the user's email address for updating his extended public key. |
| updatexpublickeyverificationexpiry | int64 | The UNIX time (in seconds) for when the `resetpasswordverificationtoken` expires. |
| lastlogin | int64 | The UNIX timestamp of the last login date; it will be 0 if the user has not logged in before. |
| failedloginattempts | uint64 | The number of consecutive failed login attempts. |
| islocked | boolean | Whether the user account is locked due to too many failed login attempts. |
| emailnotifications | int64 | The total of the values that correspond to the [`email notification types`](#email-notification-types) which the user has opted into |
| identities | array of [`Identity`](#identity)s | Identities, both activated and deactivated, of the user. |
| invoices | array of [`Invoice`](#invoice)s | Invoices submitted by the user. |

### `Abridged user`

A subset of details for a user that is used for lists.

| | Type | Description |
|-|-|-|
| id | string | The unique id of the user. |
| email | string | Email address. |
| username | string | Unique username. |
| isadmin | boolean | Whether the user is an admin or not. |

### `Email notification types`

Email notifications can be sent for the following events:

| Description | Value |
|-|-|
| Invoice has been approved | `1` |
| Invoice has been rejected | `2` |
| Payment received for invoice | `4` |

### `Invoice`

| | Type | Description |
|-|-|-|
| status | number | Current status of the invoice. |
| statuschangereason | string | The reason for the current status (if applicable). |
| timestamp | number | The unix time of the last update of the invoice. |
| month | uint16 | The invoice's month represented by a number (from 1 to 12). |
| year | uint16 | The invoice's year. |
| userid | string | The ID of the user who created the invoice. |
| username | string | The username of the user who created the invoice. |
| publickey | string | The public key of the user who created the invoice. |
| signature | string | The signature of the merkle root, signed by the user who created the invoice. |
| censorshiprecord | [`censorshiprecord`](#censorship-record) | The censorship record that was created when the invoice was submitted. |
| file | [`File`](#file) | This property will only be populated for the [`Invoice details`](#invoice-details) call. |
| version | string | The current version of the invoice. |

### `Invoice review`

| | Type | Description |
|-|-|-|
| userid | string | The ID of the user who created the invoice. |
| username | string | The username of the user who created the invoice. |
| token | string | The censorship token. |
| totalhours | int64 | The total number of hours worked for this invoice. |
| totalcostusd | int64 | The total cost (in USD) billed. |
| lineitems | array of [`Invoice review line item`](invoice-review-line-item)s | The list of line items for the invoice. |

### `Invoice review line item`

| | Type | Description |
|-|-|-|
| type | string | The type of work performed. |
| subtype | string | The subtype of work, if applicable. |
| description | string | A description of the work. |
| proposal | string | A link to a Decred proposal, if applicable. |
| hours | int64 | The number of hours spent on the work. |
| totalcost | int64 | The total cost (in USD) of the work. |

### `Invoice payment`

| | Type | Description |
|-|-|-|
| userid | string | The ID of the user who created the invoice. |
| username | string | The username of the user who created the invoice. |
| token | string | The censorship token. |
| totalhours | int64 | The total number of hours worked for this invoice. |
| totalcostusd | int64 | The total cost (in USD) billed. |
| totalcostdcr | float64 | The total cost (in DCR) billed, calculated with the provided USD/DCR rate. |
| paymentaddress | string | A Decred address generated for the user to receive the payment. |
| lineitems | array of [`Invoice review line item`](invoice-review-line-item)s | The list of line items for the invoice. |

### `Invoice review line item`

| | Type | Description |
|-|-|-|
| type | string | The type of work performed. |
| subtype | string | The subtype of work, if applicable. |
| description | string | A description of the work. |
| proposal | string | A link to a Decred proposal, if applicable. |
| hours | int64 | The number of hours spent on the work. |
| totalcost | int64 | The total cost (in USD) of the work. |

### `Identity`

| | Type | Description |
|-|-|-|
| pubkey | string | The user's public key. |
| isactive | boolean | Whether or not the identity is active. |

### `File`

| | Type | Description |
|-|-|-| |
| digest | string | Digest is a SHA256 digest of the payload. The digest shall be verified by politeiad. |
| payload | string | Payload is the actual file content. It shall be base64 encoded. Files have size limits that can be obtained via the [`Policy`](#policy) call. The server shall strictly enforce policy limits. |

### `Censorship record`

| | Type | Description |
|-|-|-|
| token | string | The token is a 32 byte random number that was assigned to identify the submitted invoice. This is the key to later retrieve the submitted invoice from the system. |
| merkle | string | Merkle root of the invoice. This is defined as the sorted digests of all files invoice files. The client should cross verify this value. |
| signature | string | Signature of byte array representations of merkle+token. The token byte array is appended to the merkle root byte array and then signed. The client should verify the signature. |

### `Login reply`

This object will be sent in the result body on a successful [`Login`](#login)
call, or as part of the [`Version`](#version) call.

| Parameter | Type | Description |
|-|-|-|
| isadmin | boolean | This indicates if the user has publish/censor privileges. |
| userid | string | Unique user identifier. |
| email | string | Current user email address. |
| username | string | Unique username. |
| publickey | string | Current public key. |
| lastlogin | int64 | The UNIX timestamp of the last login date; it will be 0 if the user has not logged in before. |

### `Invoice policy`

| Parameter | Type | Description |
|-|-|-|
| fielddelimiterchar | char | The delimiter character for fields in the invoice CSV file. |
| commentchar | char | The character denoting a comment line in the invoice CSV file. |
| fields | array of [`Invoice policy field`](#invoice-policy-field)s | A list of acceptable fields for the invoice CSV file. |

### `Invoice policy field`

| Parameter | Type | Description |
|-|-|-|
| name | string | The field name |
| type | number | The [field type](#invoice-policy-field-type) |
| required | boolean | Whether the field is required to be populated in the invoice |

### `Invoice policy field type`

| Type | Value |
|-|-|
| InvoiceFieldTypeInvalid | `0` |
| InvoiceFieldTypeString | `1` |
| InvoiceFieldTypeUint | `2` |
