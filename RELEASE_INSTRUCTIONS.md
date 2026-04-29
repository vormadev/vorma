# Release Instructions

1. Log in to npm:

```sh
npm login
```

2. Prepare the release:

```sh
make prepare-release
```

This updates `internal/pkg/npm/package.json` and
`internal/pkg/npm/vorma/create/package.json`, runs the full repo gate, and
prints the exact npm publish commands to run manually.

3. Commit and push the prepared release:

```sh
git add .
git commit -m 'v0.0.0-pre.0' --no-verify
git push
```

4. Run the printed `npm publish` commands directly from your terminal.

5. Publish the Go module after npm publish succeeds:

```sh
make publish-go
```
