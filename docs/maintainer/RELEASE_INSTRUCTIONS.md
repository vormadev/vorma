# Release Instructions

First, bump all crate versions in repo-root `Cargo.toml`.

Then run the following:

```sh
# Run gate
make gate

# Run dependency policy checks
cargo audit
cargo deny check licenses bans sources advisories

# Publish crates
cargo publish -p vorma-matcher --dry-run
cargo publish -p vorma-matcher

cargo publish -p vorma-tasks --dry-run
cargo publish -p vorma-tasks

cargo publish -p vorma-macros --dry-run
cargo publish -p vorma-macros

cargo publish -p vorma-contract --dry-run
cargo publish -p vorma-contract

cargo publish -p vorma --dry-run
cargo publish -p vorma

cargo publish -p vorma-build --dry-run
cargo publish -p vorma-build

# Publish npm packages
npm login
make ts-publish # infers version from Cargo.toml
```
