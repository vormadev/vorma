## Go rules

Any free-floating function that takes in a non-primitive type should actually be
a method on that type. For example, `whatever(my_type{})` should really always
be `my_type{}.whatever`.

---

## TypeScript rules

Always use curlies in control flow branches.

---

Always use curlies and explicit return statement for (1) object literals and (2)
anything that doesn't fit on one line.

---

## Universal rules (applicable to both Go and TypeScript)

Before writing tests or running commands, make sure to read both
`TEST_README.md` and `Makefile` in the repo root.

---

Single-use helpers are strictly prohibited unless they dramatically and
objectively simplify the code.

---

All internal symbols shall be `snake_case`, and all public-facing symbols shall
be either: (1) `SCREAMING_CASE`, (2) `camelCase`, or (3) `PascalCase`, as is
appropriate or idiomatic contextually. TypeScript types, however, should still
always be `PascalCase`, regardless of public exposure.

---

Avoid inlining magic strings.

Use a `constants.{go,ts}` file when the constants are shared across multiple
source files or packages, especially publicly observable strings such as
filenames, storage keys, header names, env var names, route names, and protocol
values.

Do not create a `constants.{go,ts}` file just because a constant is exported. If
a constant is only used by one source file, keep it in that file near the API or
logic it belongs to, even when the constant is public.

---

Comments shall be in one of the following formats only:

```
/*
Multi-line comment
*/

/////////////////////////////////////////////////////////////////////
/////// Topic
/////////////////////////////////////////////////////////////////////

/////// Sub-Topic

/// Sub-Sub-Topic

// Normal old single-line comment
```
