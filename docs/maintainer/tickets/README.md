# Maintainer Tickets

This directory is the forward-looking maintainer backlog.

Tickets are living notes for work we may want to do. Each ticket is a directory. The
canonical task note is `__TICKET.md`; anything else in the directory is task-local
scratch/context and can take whatever shape is useful.

Tickets may be nested without limit. A ticket directory may contain sub-ticket
directories, and those may contain their own sub-ticket directories, each with its own
`__TICKET.md`. Use nesting when a task naturally decomposes into smaller tasks that belong
under the parent. Keep `NEXT.md` focused on the current sequence, not on mirroring the
full tree.

Ticket notes are intentionally loose: use whatever structure makes the task clear. Some
tickets need evidence and verification notes; some need design context; some only need a
crisp reminder.

Rules:

- Add a ticket whenever future work or ideas should not be lost, even if it is unrelated
  to the current task.
- When in doubt, add or update a ticket. Tickets are the default place for future cleanup,
  follow-up work, design questions, and implementation ideas discovered during unrelated
  work.
- Use clear slug directory names. Directory order is not sequencing or priority.
- Write tickets for a cold agent with no conversation context. A ticket must contain
  enough concrete background, current facts, file paths, constraints, desired outcome, and
  verification expectations for a future agent to understand and implement the task
  without knowing who "we" are or what was discussed in chat.
- Do not assume the implementer has read prior conversations, remembers historical notes,
  or knows why the task exists. If that context matters, put the relevant facts directly
  in the ticket or in task-local context files beside it.
- Put status or progress notes inside the ticket when useful.
- `NEXT.md` is the current recommended sequence. It does not need to list every ticket.
- When a ticket or nested ticket is handled, delete its directory and remove it from
  `NEXT.md` if it is listed there.
