VORMA TESTS / SPECS

- Are the specs indeed comprehensive?
- Are the new tests highly duplicative of the old tests?
- Do the new tests in fact cover everything the old tests do?
- There are a lot of very annoying bugs enshrined in the old client tests. Are
  we covering them all?

- Anywhere we are leaking internal implementation details (non-observable
  behavior) into specs or tests?
- Anywhere we are getting overly sclerotic?

- Anything suspicious in the specs that looks like it may be accidentally
  enshrined bugs, poor designs, or unhandled edge cases?
