# Windows Child Exit Watcher

Status: blocked until Windows is a supported and testable dev target

Unix/macOS unexpected child-exit monitoring exists. Windows has intentionally lagged
behind because untested Windows process-waiting code is worse than the current degraded
next-request failure mode.

Do not land this from theory. Implement it only when the behavior can be verified on a
real Windows target or an equivalent trustworthy test setup.

What to do:

- Study the current Unix/macOS child-exit watcher and process-group behavior.
- Design the Windows equivalent against actual Windows process semantics.
- Add tests or a trustworthy verification harness that runs on Windows.
- Make the Windows dev-loop failure mode at least as clear as the Unix/macOS path.

Done means:

- Windows process-group / child-exit behavior is implemented intentionally.
- The behavior is tested on Windows.
- The dev-loop failure mode is at least as clear as the Unix/macOS path.
