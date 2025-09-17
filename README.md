Run with:

```
CGO_ENABLED=1 CC="zig cc" CXX="zig cc" go run .
```

Available options:

To use a different environment file:
```
CGO_ENABLED=1 CC="zig cc" CXX="zig cc" go run . --env-file=production.env
```

To disable TLS encryption for SMTP (dangerous, for testing only):
```
CGO_ENABLED=1 CC="zig cc" CXX="zig cc" go run . --insecure
```

To run in interactive mode with stdin commands:
```
CGO_ENABLED=1 CC="zig cc" CXX="zig cc" go run . --interactive
```

Interactive mode commands:
- `reset` - Clear all data from storage
- `exit` - Exit interactive mode

Options can be combined:
```
CGO_ENABLED=1 CC="zig cc" CXX="zig cc" go run . --env-file=test.env --insecure --interactive
```
