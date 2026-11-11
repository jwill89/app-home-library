# Plan: sign-in on huma

Written 2026-11-09 against Home Library 1.1.2, on huma v2.40.0, with Senpan's release C (5.0.0, phase 2 being built now) as the reference. A plan only: nothing here is built yet.

## The decision

Home Library runs its API on huma under `/api/v1`, but signs in and out through plain `net/http` handlers under `/auth/*`, outside huma and outside the API document. Huma doesn't ask for that: its docs show operations setting cookies and authenticating by middleware. The standard (`standards/api-and-realtime.md` line 24) puts credential and session endpoints inside the API as its one action-shaped exception. Decided by the owner 2026-11-09: sign-in moves inside huma in all three projects, under `/api/v1/auth/*`, in the committed `openapi.json`, called through the typed client. Senpan does it first and is the reference.

Recommended here:

| Question | Recommendation |
|---|---|
| Discord's callback | `/api/v1/auth/discord/callback`, versioned with the rest of the API (see Redirect flows on huma). |
| What stays outside huma | Home Library's share links (`/s/{token}`) and calendar feed (`/calendar/{file}`); `/healthz`, `/media/` and the SPA. |
| Old paths | Home Library needs none. |
| Order | Senpan 5.0.0, then Home Library, then Kindred Nights, each its own release on its owner's go. |

## What Senpan already settles

Senpan's `backend/internal/api/signin.go`, `passkeys.go` and `me.go` are the shapes to copy where a flow is the same:

- The session works inside an operation: the server wraps the mux in scs's `LoadAndSave`, so the operation's context carries the session, and `RenewToken`, `Put`, `PopString` and `Destroy` work on it. No `Set-Cookie` output field.
- What an operation can't read for itself comes through the context: `api.WithRequest(ctx, api.Request{IP, Site, ByToken})` from the server's session layer, read with `requestOf(ctx)` (`api.go:79-103`). `Site` is WebAuthn's relying party (`webAuthn(ctx)`, `passkeys.go:305`).
- Operation ids, tags and statuses: `sign-in`, `sign-out` (204), `sign-up` (202) under the tag `Sign-in`; `begin-passkey-sign-in` and `finish-passkey-sign-in` (200 who am I; 400 no ceremony under way; 401 refused; 429); `list-passkeys`, `begin-passkey`, `add-passkey` (201; 400 not accepted; 409 already added) and `delete-passkey` under `Account`.
- WebAuthn without an `*http.Request`: go-webauthn v0.18.2's `protocol.ParseCredentialRequestResponseBytes` with `ValidatePasskeyLogin`, and `ParseCredentialCreationResponseBytes` with `CreateCredential`. Home Library is on the same v0.18.2.
- Options go out as `{publicKey}`, a `json.RawMessage` (schema `{}`); the browser's answer comes in as `{credential}`, also a `json.RawMessage`. Senpan's code reads `in.Body.Credential` and declares no `RawBody`; the projects follow the code.
- A limit inside an operation answers `huma.ErrorWithHeaders(huma.Error429TooManyRequests(...), Retry-After)` (Senpan's `tooMany`, `api.go:304`), and the operation lists 429 in `Errors`.
- Every way of signing in ends in one function: Senpan's `startSession` (`signin.go:172`), Home Library's `auth.SignIn` (`internal/auth/auth.go:155-167`).

## Home Library

One owner signs in with passkeys alone; visitors come by share links. The first passkey on each of its two names is made with a setup code, so adding a passkey is open to a device that isn't signed in yet. Otherwise the flows are Senpan's, so the ids and statuses are Senpan's. Adding a passkey goes under its existing `/passkeys` resource (`list-passkeys` and `delete-passkey`, `internal/api/account.go:60` and `:75`), not `/me/passkeys`. Home Library tags no operation, so these take none.

### What moves

1. `POST /auth/passkey/login/begin` (`internal/auth/passkeys.go:36`, `loginBegin` `:248`) becomes `begin-passkey-sign-in`, `POST /auth/passkey/begin`. No input. 200 `PasskeyOptions{publicKey}`; 429. No guard. Someone with no session keeps this one ten minutes, as now.
2. `POST /auth/passkey/login/finish` (`:37`, `loginFinish` `:270`) becomes `finish-passkey-sign-in`, `POST /auth/passkey/finish`. Body `{credential}`. 200 with get-me's body (`role`, `name`, `sees`); 400 no sign-in under way; 401 refused; 429. The resolver ran before the operation, so the answer is built from the user just signed in, not the context's principal.
3. `POST /auth/logout` (`:38`, `logout` `:310`) becomes `sign-out`, 204, as Senpan's.
4. `POST /auth/setup` (`:33`, `redeem` `:137`) becomes `redeem-setup-code`, `POST /auth/setup`. Body `{code}`, required, with the standard's pattern for required text. 204 (today 200 with `{ok: true}`, which nothing reads); 401 wrong, used or expired; 422 empty (today 400); 429. Senpan has no counterpart.
5. `POST /auth/passkey/register/begin` (`:34`, `registerBegin` `:159`) becomes `begin-passkey`, `POST /passkeys/begin`. 200 `PasskeyOptions`. Guard: a new `auth.RequireRegistrant`, passing the owner or a session holding a redeemed setup code; 401 for no one, 403 for a visitor without a code (today 401).
6. `POST /auth/passkey/register/finish?name=` (`:35`, `registerFinish` `:198`) becomes `add-passkey`, `POST /passkeys`. Body `{name, credential}`, the name optional ("Passkey" when left out) with `maxLength` 60. 201 the `Passkey`; 400 no ceremony or not accepted; 409 already added (`credential_id` is unique); 401 and 403 as for begin; 422 for a name over 60, which today is cut to 60 quietly (api-and-realtime.md line 38 rules that out). A setup-code device is signed in by it, as now.

The handlers become functions over a context and bytes in `internal/auth`; the operations go in `internal/api/signin.go` (`/auth/*`) and `account.go` (the two `/passkeys` writes), and `api.Deps` gains the session manager. `knownHost` (`internal/server/server.go:154-162`) already maps the Host to its configured origin: it puts it in the context as Senpan's `api.Request.Site`, which the operations build the relying party from. The four 421 answers in `relyingParty`'s callers (`passkeys.go:167`, `:207`, `:251`, `:278`) go, since `knownHost` refuses an unknown name first.

### What else changes

- Session and resolver stay on every path (`server.go:117`): `/s/{token}` starts a session, and `/media/` checks the reader.
- Rate limit: `signInKey` (`server.go:178-183`) covers `/api/v1/auth/` (sign-out excepted), `/s/` and `/calendar/`. Registration moves to the write limit, behind its guard. The limiter's body is already problem details with `Retry-After: 10` (`internal/ratelimit/ratelimit.go:80-83`); each operation it covers lists 429 in `Errors` and says "with Retry-After" in its description, as Senpan's do. CSRF (`server.go:119`) is unchanged.
- `securityHeaders`' `/auth/` branch (`server.go:212`) goes: `/api/` already sends `no-store`.
- Problem helpers that go: `problem` and `writeJSON` (`internal/auth/auth.go:247-257`) and `Passkeys.fail` (`passkeys.go:318-321`). Huma's errors replace them.
- SPA: `authPost` (`web/src/app/session.ts:12-25`) goes. `signIn`, `addPasskey`, `redeem` and `signOut` call `api.POST` and throw the problem body, which `messageOf` (`app/problem.ts:6`) reads as now. `signIn` takes the role, name and sees from finish's answer, with no second `load()`. `addPasskey` sends the name in the body; SetupView's name field gets `maxlength="60"`. `publicKey` arrives typed `unknown` (schema `{}`) and is cast once to simplewebauthn's JSON types. `vite.config.ts:21` drops `/auth`.
- The typed client's 401 handler (`app/router.ts:200-210`) skips sign-in's answers: openapi-fetch 0.17.0 gives middleware the `schemaPath`, and one starting `/auth/` returns early. Without it, a visitor who tries the owner's sign-in and is refused, or types a wrong setup code, is sent to `/link-ended` with their session forgotten. Item 5's 403 keeps registration from tripping it for a visitor.

### What stays outside huma

- `/s/{token}` (`internal/auth/shares.go:230-265`) is a link the owner hands to people and they open as a page. Its job is to move the secret out of the address bar into a session by a 303 (security.md line 67); it has no JSON and no client. Its address is in messages already sent and can never change, so it can't carry the API's version, and the vhost's no-log rule (`deploy/apache-vhost.conf:54`) matches it.
- `/calendar/{file}` (`server.go:107`, `:286-309`) is a feed a phone's calendar subscribes to and polls. It answers `text/calendar`, and a calendar can't sign in or send a header, so the token is the path. The address lives in the owner's phone; moving it breaks the subscription.
- Discord's start and callback are navigations too, but they are the API's own sign-in steps, reached only from the SPA's button and from the address registered with Discord, and no person keeps them. Share and calendar links are addresses given out, like `/media/`, and outlive any version of the API. Both stay, with the session and the sign-in limit as now. The operations that make them (`create-share`, `create-calendar-link`) are in the document already, and their descriptions say what the link looks like.

### Tests

None of the ceremonies has a server test today; `internal/server/server_test.go` covers the share links and the calendar alone.

1. `redeem-setup-code`: a wrong code 401, an empty one 422, a right one 204, after which `begin-passkey` answers 200 with `publicKey.rp.id` the test's host name; the same code again 401.
2. `begin-passkey`: 401 for no one, 403 for a visitor by a share link.
3. `begin-passkey-sign-in` answers 200 with a challenge; `finish-passkey-sign-in` with no begin 400, and after a begin with a credential that isn't one 401.
4. `sign-out` answers 204, and `get-me` then answers role `none`.
5. The limiter: the eleventh quick begin from one address 429 with `Retry-After`; sign-out isn't counted.
6. The document has the six operations with their statuses.
7. End to end, in Chromium with a virtual authenticator (the DevTools protocol's `WebAuthn.addVirtualAuthenticator`), in a fresh context: a code from `homelibrary setup-code` (`web/e2e/server.ts:51` already runs the binary for `dev-session`), Set up this device, sign out, sign in. It runs the real ceremony, the bytes parsing included, which a Go test can't without a software authenticator.
8. A Vitest spec for the 401 handler, pulled into a function it can call: a 401 from `/auth/*` leaves the session as it was.

### Deploy day, owner only

Nothing outside the app changes. Passkeys belong to a name, not a path; the cookie and the sessions are as they were; share links and the calendar feed stay. After `task deploy`, reload the installed app on each device: an app left open keeps the old bundle, whose sign-in calls the old paths and fails with a message. No 308s: the bodies changed too, so a redirect wouldn't save an old bundle, and the owner is the only one signing in. `/sign-in` and `/setup` are the SPA's pages and don't move.

### Risks

- The 401 handler, above, and its test.
- The setup device's ten-minute session (`unfinished`, `auth.go:37`): redeem sets its deadline on the operation's context, and test 1 checks begin still works after it.
- Removing the last passkey is allowed today (`account.go:75-88`), where security.md line 50 asks for 409. The host's `homelibrary setup-code` is the way back. It isn't this release's: AGENTS.md lists it as a departure or it's fixed on its own.

### Docs

- `docs/decisions.md`: a 2026-11-09 entry: sign-in inside huma, the six operations, adding a passkey under `/passkeys`, and why `/s/` and `/calendar/` stay outside.
- `AGENTS.md` doesn't describe `/auth`. Its Sessions departure (line 88) is out of date and can go while there: `auth.go:50-57` keeps tokens hashed and names the cookie `__Host-hl_session` over HTTPS.
- `session.ts`'s comment that the ceremonies live outside the API goes with `authPost`.
- `CHANGELOG.md`.

## Redirect flows on huma

What huma v2.40.0 offers, from `docs/docs/features/response-outputs.md` in the module cache: a dynamic status is a `Status int` field on the output, "not recommended" but there (lines 32-59), and a header is a field tagged `header:"..."` (lines 61-81). The docs have no redirect example. The source documents only the operation's `DefaultStatus`, with the output's header fields under it (`huma.go:1761-1815`); the values a `Status` field takes aren't documented (`huma.go:1703-1711`, a TODO). So a redirect operation:

```go
type redirectOutput struct {
    Status   int
    Location string `header:"Location" doc:"Where the browser goes next."`
}

huma.Register(api, huma.Operation{
    OperationID:        "finish-discord-sign-in",
    Method:             http.MethodGet,
    Path:               "/auth/discord/callback",
    Summary:            "Finish signing in with Discord",
    Tags:               []string{"sign-in"},
    DefaultStatus:      http.StatusFound,
    SkipValidateParams: true,
}, h.finishDiscordSignIn)
```

1. `DefaultStatus` is 302, and `Status` is set to 302 on every answer (a zero panics, api-and-realtime.md line 88), so the document's one 302 with its `Location` is everything the operation sends.
2. The handler never returns an error. A failure is a 302 to the sign-in page with `?error=` and a reason the page knows; an error returned would be a problem-details body on a browser tab.
3. Inputs are optional strings with no constraint, and `SkipValidateParams` is set, so huma never answers 422 before the handler.
4. A rate limit's refusal is a redirect too (`?error=busy`), never a 429.
5. The OAuth state and the `next` path wait in the session between the two steps. The state answers once (`PopString`), and `next` is checked on the server (security.md line 101; Kindred's `isRelativePath`).
6. The session comes from the operation's context, as for every sign-in operation.

The callback's path: `/api/v1/auth/discord/callback`. An unversioned `/api/auth/...` would need a second huma API, with a second document, or a route outside huma, which is the split this plan removes. Under `/api/v1` it is one more operation in the one document. The registered URL then changes only with the API's major version, and a `/api/v2` keeps v1's callback until the Discord application has moved, as the standard keeps any old path until its clients have moved (api-and-realtime.md line 40).

## Adding Discord sign-in to Home Library later

It doesn't argue for keeping sign-in outside huma. What pulled OAuth outside an API framework was that it answers a browser with redirects, and the pattern above does that inside huma, with the session in the operation's context either way. What it argues for is the redirect pattern settled once in the standard before anyone else builds it, a callback path that never moves, since Discord registers it, and an account model where a way to sign in is a row, not a column.

### The account model

- An `identities` table: `id`, `user_id` (cascading), `provider` (`discord`), `subject` (Discord's user id), `name` (shown on the account page), `created_at`, `last_used_at`, and `UNIQUE (provider, subject)`. Linking adds a row, unlinking deletes one, and signing in looks up `(provider, subject)`.
- Linking happens only from a signed-in session (the account page's Link Discord, through the same start and callback with a note in the session that it's a link), never by matching a Discord name to a username: anyone can choose a Discord name.
- The session names the way it signed in, as Home Library's `sessionPasskey` does for passkeys (`auth.go:28`), so unlinking signs out the devices that came in by it (security.md line 36).
- Home Library answers on two names and builds nothing from one base URL: the Discord application registers a callback for each name, the start builds `redirect_uri` from the request's own origin, and the callback returns to the name the sign-in began on, where its cookie is.

### The standard's rules that bite (security.md)

- Line 50: removing the last way to sign in is refused with 409. Home Library has no password, so its owner's ways are passkeys and identities, and `delete-passkey` doesn't check yet.
- Line 101: `next` is checked on the server and kept in the session across Discord's round trip.
- Lines 54 and 55: the `identify` scope only, so there is no email to link by; claims read strictly.
- Line 46: an account made through Discord starts with no rights.
- Line 32: signing in moves the session to a new id, in the one sign-in function.
- Line 53: adding a passkey asks for the password when the account has one (Senpan's `begin-passkey` does), so an account made through Discord adds one without.
- Line 117: Turnstile can't ride a navigation. Discord's own consent stands in for it on the start, and the per-address limit still applies.

## The standard's edit

shared-tooling 2.7.0 puts these rules in `standards/api-and-realtime.md`: credential operations under `/api/v1/auth/*`, the rule for operations a browser navigates to, a 401 from `/api/v1/auth/*` staying with the page, and `/api/v1` as a Go service's base path.

## Order and effort

<!-- markdownlint-disable-next-line MD029 -->
2. Home Library (1.2.0) next: six operations, most of them ports of Senpan's fresh `passkeys.go`; the session store, two views and the 401 handler; six server tests and the end-to-end ceremony. About a day, with nothing for the owner outside the app.

Each ships as its own release, deployed on its owner's go; none waits on another but for the standard's edit. This project's `plans/shared-tooling-rollout.md` gains a line for the release when it's scheduled.
