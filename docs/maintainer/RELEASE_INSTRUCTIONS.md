# Release Instructions

Bump all crate versions in repo-root `Cargo.toml`.

```sh
# Run gate and publish crates
make gate && cargo publish --dry-run && cargo publish

# Publish npm packages
npm login
# If pre:
make ts-publish-pre version=0.0.0 pre=0
# Else if non-pre:
make ts-publish version=0.0.0
```
