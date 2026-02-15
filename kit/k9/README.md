# k9

```go
import "github.com/vormadev/vorma/kit/k9"
```

Compact, k-sortable IDs with a human-friendly string form.

- Binary ID size: `16` bytes (`k9.IDSize`)
- String size: `26` chars (lowercase base32, no padding)

Layout:

- High 52 bits: Unix timestamp in 100-microsecond ticks (big-endian)
- Low 76 bits: cryptographically random entropy

Byte-level split:

- Bytes `0..5` + high nibble of byte `6`: 52-bit timestamp
- Low nibble of byte `6` + bytes `7..15`: 76-bit randomness

## Why This Layout

This layout balances compactness, sortability, and readability:

- Same binary payload size as UUIDv7 (`16` bytes)
- Shorter text form (`26` chars vs UUID canonical `36`)
- More random entropy (`76` bits vs UUIDv7 `74`)
- Better time precision (`100µs` vs UUIDv7 `1ms`, 10x finer)
- Longer timestamp horizon (year `16241` vs UUIDv7 year `10889`, +`5352` years)

## Practical Benefits

- K-sortable raw bytes improve index locality and make time-range scans/cursor
  pagination straightforward.
- 26-char lowercase base32 strings are shorter and cleaner in logs/URLs than
  canonical UUID strings (36 chars with hyphens).
- XOR mixing before encoding improves visual differentiation between nearby IDs
  without losing round-trip fidelity.

## k9 vs UUIDv7

| Property                 | k9                                | UUIDv7                                  |
| ------------------------ | --------------------------------- | --------------------------------------- |
| Binary size              | 16 bytes                          | 16 bytes                                |
| Canonical string size    | 26 chars                          | 36 chars (`8-4-4-4-12` with hyphens)    |
| Timestamp precision      | 100µs ticks (52-bit field)        | Milliseconds (48-bit field, RFC 9562)   |
| Random entropy           | 76 bits                           | 74 bits (`rand_a` + `rand_b`, RFC 9562) |
| String alphabet          | `a-z2-7` (base32, no punctuation) | Hex + hyphens                           |
| String sort == time sort | No (by design, XOR-mixed prefix)  | Usually yes in canonical text form      |

Timestamp horizon (from Unix epoch):

- `k9`: through `16241-05-04T13:38:57.0495Z` (year `16241`)
- UUIDv7: through `10889-08-02T05:31:50.655Z`

## Example Strings

Sequential `k9` strings from one run:

```text
5cbjio47mq56tazuwwdfovdrpa
ksl2la6ljiyvlfqfbxjh44lp2e
qh3c2pxetnvib54nwd624tfo7y
orcvxlrpmughkrh3ea3fb4wggu
uo7flq3vpas2fp7vjvwe55rs2y
```

Sequential UUIDv7 canonical strings from one run:

```text
019c3416-8ebf-77db-b919-cd0421bd52d7
019c3416-8ec0-7d75-bba7-8735991da697
019c3416-8ec2-73e8-9980-64965c007d23
019c3416-8ec3-7966-9258-0cbff8468bc5
019c3416-8ec4-7eec-a276-02f1ce6dd450
```

## Important Ordering Note

For DB queries that require time ordering or range scans, store the raw 16-byte
ID (`BYTEA`/`BINARY(16)` style), not the 26-char string.

The encoded string is for display, logs, URLs, and APIs; because of the XOR
mixing step, lexical string order is not timestamp order.

## Security

`k9` uses cryptographically secure randomness for its random suffix, but it is
an **identifier format**, not a secret/token format.

Safe/common uses:

- Public object identifiers (users, orgs, orders, jobs, events)
- IDs that should be hard to guess at scale

Not appropriate uses:

- API keys, bearer tokens, password reset tokens, session secrets
- Contexts that require opaque, non-structured secrets
- Contexts where revealing creation-time information is unacceptable

Important:

- The timestamp is encoded in the ID and is recoverable.
- The XOR step is **not** encryption or obfuscation for security; it is only a
  human-friendliness transform to make nearby IDs easier to distinguish.

Secret token generation should use a separate high-entropy random value
(typically 128+ random bits); `k9` IDs should not be reused as secrets.

## Basic Usage

```go
id, err := k9.New()
if err != nil {
	return err
}

s := id.Serialize() // 26-char lowercase base32

parsed, err := k9.Parse(s)
if err != nil {
	return err
}

createdAt := parsed.CreationTime()
_ = createdAt
```

Batch creation with unified error handling:

```go
ids, err := k9.NewMulti(3) // e.g. userID, orgID, inviteID
if err != nil {
	return err
}
userID := ids[0]
orgID := ids[1]
inviteID := ids[2]
```

IDs returned from a single `k9.NewMulti` call share the same timestamp.

Inclusive time-range query bounds:

```go
start := time.Date(2026, 2, 6, 0, 0, 0, 0, time.UTC)
end := time.Date(2026, 2, 7, 0, 0, 0, 0, time.UTC)
startID := k9.MinIDAtTime(start)
endID := k9.MaxIDAtTime(end)
// WHERE id >= startID AND id <= endID
```

## Constants

- Exported constant: `k9.IDSize` (`16`).
- `k9.ID` is a fixed-size `[16]byte` type.

## Public API Summary

- Generate IDs: `k9.New()`, `k9.NewMulti(n)`.
- Encode/decode text: `k9.Serialize(id)`, `id.Serialize()`, `k9.Parse(str)`.
- Convert raw bytes: `k9.FromBytes(raw)`.
- Read time metadata: `k9.ToUnixMicro(id)`, `id.ToUnixMicro()`,
  `k9.CreationTime(id)`, `id.CreationTime()`.
- Build range bounds for scans: `k9.MinIDAtTime(t)`, `k9.MaxIDAtTime(t)`.
- Validation errors: `k9.ErrInvalidCount`, `k9.ErrInvalidTextLength`,
  `k9.ErrInvalidIDLength`.

## Validation Behavior

- `k9.Parse(str)` requires exactly 26 chars, otherwise returns
  `k9.ErrInvalidTextLength`.
- `k9.FromBytes(raw)` requires exactly `k9.IDSize` bytes, otherwise returns
  `k9.ErrInvalidIDLength`.
- `k9` timestamps are stored at 100µs resolution (`ToUnixMicro` always returns a
  multiple of 100).

## Why Is It Called `k9`?

Because the byte representation is k-sortable, and it contains over 9 bytes of
entropy (9.5, to be exact).
