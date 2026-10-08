# Authentication

There are two authentication methods. Each one of them has its own capabilities and specification. Adding another authentication method is described in [Building File Browser](../CONTRIBUTING.md#authentication-provider).

## JSON Auth (default)

We call it JSON Authentication but it is just the default authentication method and the one that is provided by default if you don't make any changes. It is set by default, but if you've made changes before you can revert to using JSON auth:

```sh
filebrowser config set --auth.method=json
```

Gezgin removed File Browser's reCAPTCHA option and its `hook` and `noauth` methods. A database that
still names one of those methods is refused at start-up; choose another one with
`filebrowser config set --auth.method=json` (or `proxy`).

### Login attempt limit

Every password login is counted, before the password is checked, against two budgets of the client's
address (the connection's address; forwarded headers are not trusted):

- 5 attempts for one username from one address,
- 20 attempts from one address, whatever the username,

within a sliding window of 15 minutes. Once a budget is spent the login answers `429 Too Many Requests`
with a `Retry-After` header, without checking the password. A successful login clears the budget of its
username and takes its own attempt back from the address's. The counters live in memory and start
again when the server restarts.

### Sessions

Every token carries the user's security stamp. The stamp changes, and every session issued before it
ends, when:

- the user's password changes (from the profile, the admin's user page or `filebrowser users update --password`),
- "Close all sessions" is used, in the profile or by an admin on the user's page
  (`DELETE /api/users/{id}/sessions`).

A deleted user's tokens are refused as well. Logging out only forgets the token in the browser.

### First login

When quick setup generates the admin's password, the password is written to the log and the admin
must choose a new one at the first login: until then every request except renewing the token and the
user's own password change is refused, and the new password may not be the generated one.

## Proxy Header

If you have a reverse proxy you want to use to login your users, you do it via our `proxy` authentication method. To configure this method, your proxy must send an HTTP header containing the username of the logged in user:

```sh
filebrowser config set --auth.method=proxy --auth.header=X-My-Header
```

Where `X-My-Header` is the HTTP header provided by your proxy with the username.

> [!WARNING]
> 
> File Browser will blindly trust the provided header. If the proxy can be bypassed, an attacker could simply attach the header and get admin access. Please ensure that File Browser is not accessible from untrusted networks, and that the proxy is correctly configured to strip/overwrite the header from client requests.
