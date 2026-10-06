# notifications

- Subdomain: generic
- Hosted in: `services/api` (module `contexts/notifications`)

## Purpose

Tell the people running The Bakery when something needs their attention: a
Deployment or a Backup execution failed, a Server became unreachable or came back, a
Server's disk usage is high. Each Notification goes to every Notification
channel subscribed to its Event kind: an email address, a Discord or Slack
channel, a Telegram chat, a Pushover user, an ntfy topic or a webhook. It also emails an
Invitation to the person invited.

It does **not** decide what is worth telling: deployments, databases,
servers and identity announce what happened, and this context only turns
that into a message. It keeps no history beyond the recent Deliveries of
each channel; an audit log is something else.

## Language

| Term | Meaning |
| ---- | ------- |
| Notification channel | A named place Notifications go to: a Channel kind, its settings (secrets encrypted), the Event kinds it is subscribed to, and whether it is enabled or disabled. |
| Channel kind | `email` (SMTP server, from address and optional from name, recipients, an optional timeout and EHLO domain), `discord` (an incoming webhook URL, a secret, and whether Critical event mention is on: `@here` on an alarming Event kind), `slack` (an incoming webhook URL, a secret), `telegram` (a bot token, a chat id and optionally a forum topic, a message thread id, per Event kind), `pushover` (a user key and an API token, both secrets), `ntfy` (a server URL, a topic and optionally a token) or `webhook` (any URL, kept secret, JSON signed with an optional secret). |
| Event kind | What a Notification is about: `deployment_failure`, `deployment_success`, `backup_failure`, `backup_success`, `server_unreachable`, `server_reachable`, `server_disk_usage`. A new channel is subscribed to all but the two `_success` ones. |
| Notification | What a publisher hands over: an Event kind, a title, a body, an optional link into the dashboard and when it happened. Not stored on its own. |
| Delivery | One Notification sent to one Notification channel: `pending`, `sent` or `failed`, the attempts made (at most 3) and the last error. The 50 newest per channel are kept. |
| Test notification | A Notification sent at once from a channel's Send test button, recorded as a Delivery like any other. An email channel's can go to one typed recipient instead of its own. |

## Model

### Aggregates

| Aggregate | Invariants |
| --------- | ---------- |
| Notification channel | The name is unique (1–63 characters). The settings are valid for the Channel kind: URLs are `http` or `https`; an email channel has a host, a port, a security mode (`none`, `starttls`, `tls`), a from address and at least one recipient; Telegram has a bot token and a chat id, and its topics are digits, each for a known Event kind; Pushover has a user key and an API token; ntfy a server URL and a topic. An email channel's timeout is 1–300 seconds (empty means 30) and its EHLO domain a host name. It is subscribed to at least one Event kind. A secret setting left empty on a change keeps its stored value. A new channel is enabled; a disabled one gets no Deliveries and cannot be tested. |
| Delivery | Belongs to one Notification channel and goes when it goes. Attempts only grow, at most 3; after the third failure it is `failed` for good. Retries are 10 s after the first attempt and 60 s after the second. |

### Commands

- `AddChannel`, `ChangeChannel`, `DeleteChannel` [admin, owner]. Enabling and
  disabling is a `ChangeChannel` of its enabled flag.
- `TestChannel(channel, recipient?)` [admin, owner]: sends a Test notification
  at once and answers with the outcome. Refused for a disabled channel. An
  email channel sends it to the recipient when one is given, else to its own.
- `Notify(notification)`: a Delivery per enabled Notification channel
  subscribed to its Event kind, sent in the background. Never fails the caller.
- `SendInvitation(invitation)`: the Invitation's link by email to the invited
  person, through the enabled email channel with the lowest id; reports
  whether it was sent.

### Domain events

None published.

## Integration

- **Publishes:** the Notification channels API
  (`/api/notification-channels` and its Test and Deliveries) for the
  dashboard, admin-only; `notifications.Start(ctx)` (the dispatcher, and the
  subscriptions below). Nothing calls into it: it hears what the others
  announce.
- **Consumes**, each translated into a Notification in this context's words:
  - `deployments.OnDeploymentFinished` (succeeded or failed; not cancelled
    ones, nor those failed by a restart) → `deployment_failure` /
    `deployment_success`; a Preview Deployment's message names the
    Preview ("preview of pull request #n").
  - `databases.OnBackupExecutionFinished` (succeeded or failed; not those failed by a
    restart) → `backup_failure` / `backup_success`.
  - `servers.OnServerHealthChanged` (a Server probe changed something) →
    `server_unreachable`, `server_reachable` or `server_disk_usage`.
  - `identity.OnInvitationCreated` → an email to the invited person, whose
    outcome identity reports back to the inviting admin.
  - The auth middlewares from identity.
- **Talks to:** SMTP servers, Discord, Slack, Telegram's Bot API, Pushover's API, ntfy
  servers and webhook receivers, each over its own small sender.

## Why it's shaped this way

- **A context of its own, generic.** What goes wrong lives in four contexts;
  none of them should know about Discord. Each announces what happened in its
  own terms and this context translates, so a new Channel kind touches only
  this context and a new Event kind touches only its publisher and the
  translation here.
- **Events are callbacks the downstream registers**, like every other event in
  The Bakery (`OnApplicationDeleted`, `OnCleanup`): the publishers export
  `On…(f)` and notifications registers in `Start`. There is one process, so a
  broker would add a moving part and no decoupling the callbacks lack. The
  publishers call them in their own goroutine with a recover, so a
  subscriber never slows or breaks a Deployment, a Backup execution or a probe.
- **SMTP settings live on the email Notification channel**, not in `.env`:
  an installation is configured from the dashboard, and more than one email
  channel (a team inbox, an on-call address) is possible. Goravel's mail
  facade reads one global configuration, so the email sender speaks SMTP
  itself (`net/smtp`).
- **Deliveries are stored and retried in-process** (now, +10 s, +60 s) by a
  dispatcher that claims due ones with `FOR UPDATE SKIP LOCKED`, so a
  restart resumes them instead of losing them. The volume one installation
  produces does not justify a queue worker to operate.
- **Only the newest 50 Deliveries per channel** are kept: they answer "did
  it arrive?", not "what happened last month?".
- **Invitation emails go through the first enabled email channel**, whatever
  Event kinds it has, to the invited person only. An Invitation is addressed to a
  person, not to a channel's recipients, so it is not a Delivery; a failed
  send leaves the copy-the-link path as before and is logged.
- **The Telegram and Pushover API base URLs are configurable**
  (`BAKERY_TELEGRAM_API_URL`, `BAKERY_PUSHOVER_API_URL`) only so tests can
  point them at a local stand-in; the other kinds take full URLs already.
- **A Test notification mentions no one and goes to no forum topic**, as
  Coolify's Test does: it carries the channel's first Event kind only so its
  Delivery has one.
- **A Telegram forum topic is set per Event kind**, as Coolify sets one per
  notification: a Notification of a kind without a topic goes to the main
  chat. Coolify's topics for Event kinds The Bakery does not have yet
  (status change, restart limit, scheduled tasks, Docker cleanup, server
  patching, Traefik) come with those Event kinds.
- **Notification channels and Deliveries are The Bakery's own.** Coolify keeps
  one settings row per channel kind per team (one Discord, one Slack, ...)
  and no delivery log. The Bakery lets an admin add several named Notification
  channels of the same kind and shows each one's recent Deliveries, which
  is how a failing channel gets noticed. The Event kind values are
  Coolify's (`deployment_success`, `server_disk_usage`, ...), so the
  subscriptions map one to one. They were The Bakery's own before
  (`deployment_failed`, `deployment_succeeded`, `backup_failed`,
  `backup_succeeded`, `disk_almost_full`); stored subscriptions and
  Deliveries were migrated, but a webhook receiver that matched on the old
  names in the payload's `event` field must match the new ones. `ntfy` is a Channel kind Coolify does not
  have; it costs one small sender and is popular with self-hosters.
- **One page per Channel kind, with a picker only when a kind has several
  channels.** The Notifications pages are Coolify's, at its URLs
  (`#/notifications/{kind}`, ntfy last): a kind's page edits its one channel
  exactly as Coolify's page edits its settings row. When a kind has two or
  more channels a picker (their names, Add channel, Delete) sits above the
  page; with one, Add channel and Delete sit at its foot, so the page itself
  stays Coolify's and the several-channels case stays reachable. A kind with
  no channel shows the empty form: saving it stores the channel disabled,
  as Coolify saves settings without turning a channel on, and Enable stores
  it enabled, named after its kind. An event toggled in the events grid is
  saved at once, as Coolify's are, from the stored settings, so other edits
  not yet saved stay unsaved. The recent Deliveries sit below the events grid.
