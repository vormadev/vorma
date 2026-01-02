---
title: Why You Should Write Your Backends in Go (Not TypeScript)
description: Why you should write your backends in Go (not TypeScript)
date: Aug 30, 2025
tags: ["Go", "TypeScript"]
---

## Reason 1: Standard Library 📘

You may want your frontend to be a delicate flower of handpicked dependencies
(_I get it!_), but it’s extremely nice having everything you need in Go’s
excellent standard library on the backend.

## Reason 2: LLMs Are Better at Go 🤖

Contrary to popular opinion, LLMs are actually better at Go than TypeScript.
**_Yes, really!_** How can this be? To be sure, LLMs know TypeScript insanely
well, but the dynamic, tsconfig-driven nature of TypeScript still makes it a
moving target. Go’s type system has relatively no ambiguity in comparison. This
helps **_a lot_**.

## Reason 3: Crystal Clear Boundaries 🧼

In TypeScript, you’ll naturally want to share helpers and schemas across your
backend and frontend. While this is undoubtedly nice, it's also pernicious.
Every time you do, you risk both (A)&nbsp;subtle bugs caused by server-client
runtime differences and (B)&nbsp;leaking sensitive server code to the client.
These problem spaces are completely eliminated when you write your backend in
Go; it's quite refreshing!

## Reason 4: Performance ⚡️

You can often expect anywhere from a 100% to a 1,000% (2x to 11x) performance
improvement when you choose Go over Node.js. It obviously depends on what
specifically you are doing, but in general, your Go server is going to perform
better than a comparable Node.js server.

## Reason 5: Cost 💰

This is just the other side of the [performance](#reason-4-performance) coin.
When your operations are faster and more efficient, you can do more for the same
cost. Money money money money... MONEY.

## Reason 6: Fun 😀

Working on a feature in two languages is fun. When you write your backend in Go
and your frontend in TypeScript, switching back and forth between languages
feels almost like taking a break. Go is so simple that this hardly qualifies as
context switching, but the enjoyment that comes from a change of pace is still
there. It's nicer than you think.

## Reason 7: Stability 🪨

Go is always evolving, but compared to the JavaScript ecosystem, the commitment
to backwards compatability is breathtakingly good. Most of the churn that the
JS/TS ecosystem is infamous for is really the result of finding (_ostensibly_)
better and better ways of writing fullstack apps.

As it turns out, when you limit TypeScript to just your frontend, much of the
churn goes away. And to the extent it's still there, it's much more manageable
(and sometimes even... _fun_). All the while, Go is there handling your backend
in a UI-agnostic way, stable as a rock. It's great.

## Reason 8: Offload the TypeScript LSP 😅

When you have a large fullstack application written all in TypeScript, it can
start to really weigh down the TypeScript LSP. It gets slow, and you need to
restart it a lot. Perhaps ironically, the re-write of the TypeScript compiler to
Go will help here, but nothing is a silver bullet. Having your backend in Go
makes the TypeScript side of your application much more lightweight, and the
TypeScript LSP will thank you for it.

---

[Vorma](/) makes writing fullstack, dynamic, type-safe applications with Go
backends and TypeScript frontends insanely easy.

You can deploy Vorma apps absolutely anywhere, even on
[Vercel](https://x.com/rauchg/status/1955639485385118134).

Give it a try today:

```sh
npm create vorma@latest
```
