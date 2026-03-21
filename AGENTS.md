## Go Rules

Any free-floating function that takes in a non-primitive type should actually be
a method on that type. For example, `whatever(my_type{})` should really always
be `my_type{}.whatever`.

## TypeScript Rules

Always use curlies in control flow branches.

## All-Lang Rules (Go/TypeScript/Rust)

All internal symbols shall be `snake_case`. All public-facing symbols shall be
either: (1) `SCREAMING_CASE`, (2) `camelCase`, or (3) `PascalCase`, as is
appropriate or idiomatic for the particular use case.

---

Don't use magic strings. Further, any constants that are publicly observable
(e.g., filenames, keys, etc.) should go into a `constants.{go,ts}` file for the
applicable package so it's all quickly scannable and changeable from one place.

---

Comments shall be in one of the following four formats only:

```
/*
Multi-line comment
*/

/////////////////////////////////////////////////////////////////////
/////// Topic Header
/////////////////////////////////////////////////////////////////////

/////// Sub-Topic Header

// Single-line comment
```
