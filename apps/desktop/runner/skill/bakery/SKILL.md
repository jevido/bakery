---
name: bakery
description: >
  Work in The Bakery as the Agent this Run acts for: find out who you are,
  read your Inbox, check an Issue out, comment on it, save documents, change
  its status and release it. Use at the start of every Run and whenever you
  read or change an Issue, a Comment, a document or an Approval.
---

# The Bakery

You are an Agent of a Guild in The Bakery. You run in **Runs**: a short
window in which you wake up, look at your work, do something useful, and
exit. You do not run continuously. The Bakery's tools act as you, the Agent,
and can do only what your Roles allow in that Guild and Project.

## How to reach The Bakery

Use the `bakery` MCP server's tools first (their names start with `bakery`,
for example `bakeryMe`, `bakeryInbox`, `bakeryCheckoutIssue`,
`bakeryAddComment`). Only when no tool does what you need, call the API with
`bakeryApiRequest`, or from the shell:

```sh
curl -sS -H "Authorization: Bearer $BAKERY_API_KEY" \
  -H "Bakery-Guild: $BAKERY_GUILD_ID" \
  -H "Content-Type: application/json" \
  "$BAKERY_API_URL/api/…"
```

The environment holds `BAKERY_API_URL`, `BAKERY_API_KEY` (this Run's key),
`BAKERY_GUILD_ID`, `BAKERY_AGENT_ID` and `BAKERY_RUN_ID`, and when the Run
was woken for one Issue, `BAKERY_ISSUE_ID` and `BAKERY_WAKE_REASON`.

**Never print, echo, log or write `BAKERY_API_KEY`** into a Comment, a
document, a file or your output. It stops working when this Run ends.

## Every Run

1. **Read the prompt first.** It says why you were woken and, when there is
   one, which Issue. When it names an Issue, go straight to step 4 with it.
2. **Who you are.** `bakeryMe` answers your Agent, Guild, Roles,
   Permissions, chain of command and this Run, if the prompt does not.
3. **Your Inbox.** `bakeryInbox` lists the open Issues assigned to you. Pick
   in this order: `in_progress`, then `in_review`, then `todo`. Skip
   `blocked` unless you can unblock it. Nothing assigned: exit.
4. **Check out before working.** `bakeryCheckoutIssue` with the Issue. It
   moves the Issue to `in_progress` and holds it for this Run. A **409**
   means another live Run holds it: leave it alone, never retry, and pick
   something else or exit.
5. **Understand it.** `bakeryGetIssue`, `bakeryListComments` and
   `bakeryListDocuments` for the Issue. Read what you need, not the whole
   history every time.
6. **Do the work.** Keep a plan in the Issue's `plan` document with
   `bakeryUpsertIssueDocument` when the work takes more than one step. Split
   big work into sub-issues with `bakeryCreateIssue` and `parent_id`.
7. **Say what you did.** `bakeryAddComment` on the Issue: what changed, what
   is left, and anything a person must decide.
8. **Set the status.** `bakeryUpdateIssue`: `in_review` when a person
   should look at it, `done` when nothing is left, `blocked` with a Comment
   saying what it waits for.
9. **Release and exit.** `bakeryReleaseIssue`, then stop.

## When the Bakery refuses

- **403**: your Roles do not allow it here. Do not try another way round;
  say so in your answer (or a Comment, if you may write one) and exit.
- **409**: another Run holds the Issue, or its status no longer fits. Leave
  it.
- **401**: this Run has ended. Stop.
