---
title: Get Started
description: Get started with the Vorma framework
---

## Prerequisites

Make sure you have both `Go` (at least `v1.24`) and `Node` (at least `v22.11`)
installed on your machine.

## Step 1

Start in the directory where you want your new Vorma app to live.

Then, if you don't already have a Go module initiated, do so. You can name your
module anything you want, and it's fine if your `go.mod` file lives in a parent
directory.

```go
go mod init app
```

## Step 2

Now let's create a little bootstrapping script that can automatically build out
our Vorma project.

Start by running the following commands:

```sh
mkdir __bootstrap
touch __bootstrap/main.go
go get github.com/vormadev/vorma
```

## Step 3

Insert the following into the `__bootstrap/main.go` file you created in Step 2,
and edit the options as appropriate:

```go
package main

import "github.com/vormadev/vorma/bootstrap"

func main() {
	bootstrap.Init(bootstrap.Options{
		GoImportBase:     "app",     // e.g., "appname" or "modroot/apps/appname"
		UIVariant:        "react",   // "react", "solid", or "preact"
		JSPackageManager: "npm",     // "npm", "pnpm", "yarn", or "bun"
		DeploymentTarget: "generic", // "generic" or "vercel" (defaults to "generic")
	})
}
```

## Step 4

Now let's run the bootstrapping script. This will (i)&nbsp;create a new Vorma
project in your current working directory and (ii)&nbsp;install the required
packages for the `UIVariant` you chose (using the `JSPackageManager` you chose).

```sh
go run ./__bootstrap/main.go
```

## Step 5

Once you're done, you can delete the `__bootstrap` directory from your project:

```sh
rm -rf ./__bootstrap
```

## Step 6

Enjoy! If you have questions about how to use Vorma, make sure to first check
out the
[source code for this site](https://github.com/vormadev/vorma/tree/main/internal/site),
which shows how to do some basic things. Then, if you still have questions, feel
free to open issues in our
[GitHub repo](https://github.com/vormadev/vorma/issues) or contact us on
[X / Twitter](https://x.com/vormadev).
