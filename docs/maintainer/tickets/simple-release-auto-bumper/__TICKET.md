Add a very simple xtask that does this item of the release instructions automatically:

```
First, bump all crate versions in repo-root `Cargo.toml`.
```

It should be bone-simple and safe. If it's not safe to do, just close the ticket. Unsafe
and automatic is worse than safe and manual.

It should simply tell you what the last version was and ask you to type in the new
version. The prompt should already include the `v` so you don't need to type that in.

Something like:

```
last version: v0.99.4-pre.7
enter the new version:
v
```
