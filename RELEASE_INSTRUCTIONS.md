1. npm login

```sh
npm login
```

2. bump package.json / run prep / run build

```sh
make npmbump
```

3. publish to npm

if PRE release:

```sh
npm publish --access public --tag pre
cd vormaclient/create && npm publish --access public --tag pre && cd ../../../../
```

if FINAL release:

```sh
npm publish --access public
cd vormaclient/create && npm publish --access public && cd ../../../../
```

4. push to github

```sh
git add .
git commit -m 'v0.0.0-pre.0'
git push
```

5. publish to go proxy / push version tag

```sh
make gobump
```

6. profit
