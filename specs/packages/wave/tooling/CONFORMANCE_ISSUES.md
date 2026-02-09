# wave/tooling Conformance Issues

Status: Draft  
Last Updated: 2026-02-09  
Purpose: Track `wave/tooling` implementation divergences for build/dev control-plane contracts.

## Open `wave/tooling` Issues

| WCI ID | Affected Requirement(s) | Summary | Status |
|---|---|---|---|
| WCI-001 | WAVE-DEV-012 | Cycle-vite reload source exclusivity ambiguity in current orchestration path. | open |
| WCI-002 | WAVE-DEV-032 | Vite startup failure currently log-only continuation. | open |
| WCI-003 | WAVE-DEV-033 | Build-retry wake-up currently drops restart strength bits. | open |
| WCI-004 | WAVE-DEV-034 | App startup failure currently non-actionable continuation. | open |
| WCI-005 | WAVE-DEV-035 | Readiness failures currently warning-only continuation. | open |
| WCI-006 | WAVE-EVT-019 | Blocking event-phase failures not gating success-phase behavior. | open |
| WCI-007 | WAVE-STATIC-005, WAVE-CSS-003 | Hashed-artifact rotation cleanup failures warning-only. | open |
| WCI-008 | WAVE-EVT-023 | Concurrent restart arbitration order-sensitive downgrade risk. | open |
| WCI-009 | WAVE-DEV-036 | Config reload loses framework-injected runtime-only fields. | open |
| WCI-010 | WAVE-DEV-030 | Revalidate script path lacks rejection cleanup for overlay state. | open |
| WCI-011 | WAVE-EVT-001 | Pattern-level dedupe may drop stronger implicit work. | open |
| WCI-012 | WAVE-STATIC-015 | Granular stale-artifact removal failure silently ignored. | open |
| WCI-013 | WAVE-SCHEMA-007 | Framework schema extensions can override reserved sections. | open |
| WCI-014 | WAVE-EVT-028 | Directory watch-expansion failure silently ignored. | open |
| WCI-015 | WAVE-EVT-029 | CSS hot-reload path ignores CSS read failures. | open |
| WCI-016 | WAVE-EVT-030 | Callback restart can downgrade implicit Go recompile intent. | open |
| WCI-017 | WAVE-EVT-031 | Watch-loop channel source races mutable watcher pointer. | open |
| WCI-018 | WAVE-DEV-041 | stopApp suppresses kill/wait failures. | open |
| WCI-019 | WAVE-EVT-032 | Stale-watch removal errors dropped; tracked state unconditionally deleted. | open |
| WCI-020 | WAVE-EVT-033 | Closed error-channel receive not treated as watcher-loop shutdown. | open |
