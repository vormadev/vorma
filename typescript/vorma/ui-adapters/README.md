## ARCHITECTURE NOTES

- Tests for `ui-adapters` packages live in `vorma/client/src/tests/dist`. Do not
- Tests for `ui-adapters` packages live in `vorma/black_box_tests/dist`. Do not
  add tests here.
- Shared modules should be exported from the `client/__internal` package and
  imported that way -- not directly imported from the file system.
