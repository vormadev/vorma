"Build-time: backend package discovery" -- I feel like this is probably OK
uncached, unless we could get a big bang-for-your-buck improvement without
adding a lot of complexity.

"`discover_backend_packages` runs on every build cycle" -- same comment as
above, if there's a "quick-win" for this that doesn't introduce a lot of
complexity, i'm all for it. but if it's super complex, i doubt this is that much
of a bottleneck

"`build_dev_snapshot` is called from two places with identical logic" -- this
just depends on how much duplication and how fragile the logic is. if it's a
material amount of duplication or the logic needs to stay in sync and could
drift, that's precisely when an abstraction IS the right thing.
