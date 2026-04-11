1. prep

```sh
make prepforpub
```

2. npm login

```sh
npm login
```

3. bump package.json / run prep / run build

```sh
make npmbump
```

4. publish to npm

if PRE release:

```sh
cd internal/pkg/npm
npm publish --access public --tag pre
cd vorma/create
npm publish --access public --tag pre
cd ../../../../../
```

if FINAL release:

```sh
cd internal/pkg/npm
npm publish --access public
cd vorma/create
npm publish --access public
cd ../../../../../
```

5. push to github

```sh
git add .
git commit -m 'v0.0.0-pre.0'
git push
```

6. publish to go proxy / push version tag

```sh
make gobump
```

7. profit
