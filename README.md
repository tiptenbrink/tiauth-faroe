## WIP: Do not yet run in production!

Monorepo for the Tiauth Faroe distribution and friends.

Includes:
- Tiauth Faroe server distribution (Go files in project root)
- Python package to help develop user store in a Python project (`py-user-server`)
- JS test suite that covers all server actions (`testapp`)

## Tiauth Faroe server distribution

The Tiauth Faroe server distribution is a _distribution_ of the [Faroe project](https://github.com/faroedev/faroe) with opionated defaults.

It has the following features:
- E-mail sending support by connecting to an SMTP server (tested on Google SMTP proxy)
  - Keeps single connection alive (might be expanded to a pool in the future) to minimize e-mail sending latency
  - Connection is reestablished in case of failure
- SQLite database for user storage
- Configuration options that make it suitable for testing, such as:
  - Interactive mode with a `reset` command to clear the storage
  - Insecure mode to allow testing with a local SMTP server (see test suite section)
- Parameters (such as SMTP port and other things) can be set with an .env file (by default it looks at a file exactly called `.env`, but other files can be passed)

### Running

It relies on the [mattn/go-sqlite3](https://github.com/mattn/go-sqlite3) package for SQLite support. Therefore, running and building require enabling CGO:

```
CGO_ENABLED=1 go run .
```

For easy cross-compilation we can use Zig (todo):

```
CGO_ENABLED=1 CC="zig cc" CXX="zig cc" go run .
```

TODO:
- Do the channel errors all really work?

## Test suite

TODO:
- Add more negative tests, such as user_not_found being returned in case the counter is incorrect, etc.

This distribution has been tested with a test suite that relies on the [`@faroe/client`](https://github.com/faroedev/js-client) JavaScript package. All major server actions work. Note that this distribution assumes that an _external_ user store is used.

The test suite sets up a local SMTP server to test actions that require a verification code.

The tests can be run with (from the `testapp` directory):

```
pnpm test
```

Seemingly, there are some [file handle leaks](https://github.com/nodemailer/smtp-server/issues/232) in the `smtp-server` package used to run the SMTP server. However, this might also be caused by the weird environment of starting it using Vitest's "globalSetup". If anyone has better suggestions of how to do this, PRs are welcome.

Note that a few CLI options must be passed to the Faroe server in order for the test suite to work:

```
CGO_ENABLED=1 go run . --insecure --no-smtp-init --no-keep-alive
```

If you use the `.env.test` .env file and you also run it in interactive mode:

```
CGO_ENABLED=1 go run . --insecure --no-smtp-init --no-keep-alive --interactive --env-file .env.test
```

## Python user store lib (`py-user-server`)

The project I'm using Faroe for is a Python web server. Hence, I created a package in the same vein as the official [go-user-server](https://github.com/faroedev/go-user-server) and [js-user-server](https://github.com/faroedev/js-user-server) packages. However, since in Python both async and sync server frameworks are very common, I tried to design it in a way that mostly abstracts over this.

Instead of having a duplicate async and sync interface, we simply define a dataclass for every operation (the effects). Then the `AsyncServer` and `SyncServer` protocols can be implemented (similar to the `Server` class in `js-user-server`), with the only downside compared to e.g. `js-user-server` that you have to write a big if-elif chain to deal with all the possible effect cases. However, this means you can totally customize the function signatures however you like.

Then, the `handle_request_sync` and `handle_request_async` can be called with the server implementations to actually retrieve the result.

Just like in the `js-user-server` case, how these requests are actually received and sent is up to you. In the future I'll probably write a full reference implementation.
