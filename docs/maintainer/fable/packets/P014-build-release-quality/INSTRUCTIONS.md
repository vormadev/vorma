# P014 — vorma-build release-quality pass (docs + thermo-nuclear findings)

Read `docs/maintainer/fable/README.md` (the packet protocol binds you), then
`LEARNINGS.md`, `STATE.md`, repo-root `AGENTS.md`, `docs/maintainer/REMINDERS.md` (the
dev-loop invariants — event-driven only, least-work, Vite lifecycle rules — are
load-bearing here), and the review bar:
`docs/maintainer/skills/thermo-nuclear-system-review/SKILL.md`. Model packets:
P011/P012/P013 (same shape).

## Context

Same Phase D shape. `vorma-build` is the build/dev-loop crate: `vorma_build::run` is its
one app-facing entry (census F11); most of the surface is framework-integration. Recent
history that binds: the P001 waitid fix and P001b event-kind gate live here (their
doctrine comments are load-bearing); standing tickets
`dev-watch-excludes-not-pruned-from-os-watch` (watch-scope waste) and
`vorma-build-lib-test-flake-under-load` (now diagnosed as an `AddrInUse` port collision) —
findings should reference, not duplicate. Windows child-exit watching is a stub
(`windows-child-exit-watcher` ticket).

## Scope

1. **Doc sweep.** Every public item to the teaching bar, honest about audience
   (`vorma_build::run` app-facing; the rest framework-integration — say so). Deny
   attributes where not present; doctests only where cheap and deterministic (this crate
   spawns processes and watches filesystems — most contracts are better taught in prose;
   do not write doctests that spawn cargo or bind ports).
2. **Optional direct landing, ONLY if trivially safe:** the flake ticket's diagnosis is
   now precise (a lib test binding a fixed port under parallel load). If you can identify
   the offending test(s) and the fix is exactly switch-to-OS-assigned-port (`:0`) with no
   behavioral loss to what the test pins, land it with the ticket's verification (loop the
   suite under load), and delete the ticket. If it is anything more than that, leave the
   ticket and report.
3. **Thermo-nuclear review, FINDINGS MODE.** Direct-landable class as in P011-P013. The
   dev-loop event pipeline (watcher, classify, rebuild orchestration, process_runner) is
   semantics-frozen; REMINDERS invariants are non-negotiable.
4. **Checklist verdict** item by item with evidence.

## Hard constraints

- Dev-loop semantics FROZEN; REMINDERS invariants absolute (no polling, least-work, Vite
  restart rules). No public API changes. Tests never cheat (the flake fix, if landed, must
  not weaken what the test pins). Scoped fmt writes only; no git actions; no network
  installs; unexpectedly dirty unowned files are escalations, never cleanup.

## Definition of done

- Deny attributes green; doc build under `-D warnings` clean; any doctests pass.
- Findings report + checklist verdict; flake either fixed-with-loop-verification or left
  ticketed with what was learned.
- Gates green: workspace tests + doctests, clippy `-D warnings`, fmt check,
  `make loom-tasks` untouched-green.
- REPORT.md per template (or in-message on guardrail).
