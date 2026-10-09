# Authentication

Gezgin has one authentication method: a username and a password, which File Browser called JSON
authentication.

## JSON Auth

Gezgin removed File Browser's reCAPTCHA option and its `hook`, `noauth` and `proxy` methods. A
database that still names one of those methods is refused at start-up; switch it to JSON
authentication with:

```sh
filebrowser config set --auth.method=json
```

`proxy` trusted a header naming the user, which Gezgin cannot tell from one a client wrote itself:
anyone reaching Gezgin's port directly could have signed in as any user.

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
