# dclaude

**English** | [한국어](README.ko.md)

Per-directory Claude Code accounts. Run `dclaude init` in a directory, and Claude Code sessions started in that directory tree use that directory's login.

- **Separate:** the login and account state
- **Shared:** your global settings, `CLAUDE.md`, plugins, skills and hooks, and by default the conversation history
- **Everywhere else:** your global account, as before

> **Status:** prototype. Commands and on-disk formats may change.

## Contents

- [Why](#why)
- [Features](#features)
- [Requirements](#requirements)
- [Installation](#installation)
- [Quick start](#quick-start)
- [Commands](#commands)
- [Agent apps](#agent-apps)
- [How it works](#how-it-works)
- [Limitations](#limitations)
- [Known issues](#known-issues)
- [Troubleshooting](#troubleshooting)
- [Development](#development)
- [Roadmap](#roadmap)
- [Related projects](#related-projects)

## Why

**To separate Claude Code accounts by directory, or to pin a specific account to a directory.**

## Features

- **Directory-scoped login.** The nearest `.claude_config` at or above the current directory decides the account.
- **Your setup, another account.** A profile links `~/.claude`, so settings, `CLAUDE.md`, plugins, skills, hooks and, by default, the conversation history stay the same.
- **No token copying.** Each profile logs in with `claude auth login`. dclaude never writes or refreshes credentials.
- **Worktrees without a new login.** `dclaude ln` points a git worktree, or any other directory, at an existing profile.
- **Agent apps too.** `dclaude-code` stands in for `claude` in apps that start it without a shell. See the [apps checked so far](#agent-apps).
- **Safe `.envrc` handling.** dclaude edits only its own block and never approves content it did not write.
- **Clean removal.** `deinit` logs out and removes the Keychain items, the profile, the `.envrc` block and the links to it.
- **Status at a glance.** `info` shows the account, its 5-hour and 7-day usage, and whether the profile is active in this shell.

## Requirements

- macOS or Linux
- The [Claude Code](https://code.claude.com/docs) CLI (`claude`) on `PATH`
- [direnv](https://direnv.net), hooked into your shell. `init` and `login` stop if direnv is missing.
- Go 1.26.2 or later, only to install with `go install` or build from a clone

## Installation

1. Install dclaude, and `dclaude-code` for apps that start `claude` without a shell, such as Orca.

   With Homebrew, which installs both commands and direnv:

   ```bash
   brew install powerumc/tap/dclaude
   ```

   Without Homebrew, [install direnv](https://direnv.net/docs/installation.html) with your package manager first, then use one of these:

   - **Prebuilt binaries** for macOS and Linux (amd64, arm64), from the [releases page](https://github.com/powerumc/dclaude-config/releases). Unpack the archive for your system and put both commands in a directory on your `PATH`:

     ```bash
     tar xzf dclaude_*_darwin_arm64.tar.gz
     mkdir -p ~/.local/bin && mv dclaude dclaude-code ~/.local/bin/
     ```

     If macOS refuses to open them because the developer cannot be verified, clear the quarantine flag the browser set: `xattr -d com.apple.quarantine ~/.local/bin/dclaude ~/.local/bin/dclaude-code`.

   - **Go**:

     ```bash
     go install github.com/powerumc/dclaude-config/cmd/dclaude@latest
     go install github.com/powerumc/dclaude-config/cmd/dclaude-code@latest
     ```

   To build from a clone, see [Development](#development).

2. Hook direnv into your shell. `dclaude init` offers to do this for zsh, bash, fish and tcsh. To do it by hand, for zsh:

   ```bash
   echo 'eval "$(direnv hook zsh)"' >> ~/.zshrc   # for every new terminal
   eval "$(direnv hook zsh)"                      # for this terminal, right away
   ```

## Quick start

### A profile for a project

```bash
cd ~/workspace/hello-project
dclaude init      # creates the profile, offers to share ~/.claude and the history, then logs in
claude            # runs as the hello-project account
dclaude info      # shows the account and usage here
```

`dclaude info` inside a profile:

```text
profile   /Users/you/workspace/hello-project/.claude_config
shared    ~/.claude: everything but the login and account state
.envrc    /Users/you/workspace/hello-project/.envrc
direnv    allowed
shell     active
account   you@example.com, Example Org, max (claude.ai)
storage   Keychain: Claude Code-credentials-1a2b3c4d
usage     5h 22% (resets 10/15 17:10, in 1h 47m)
          7d 14% (resets 10/20 11:00, in 4d 19h)
```

Outside any profile, `dclaude info` shows the global account.

### Worktrees and other directories

```bash
git worktree add ../hello-project-feature
cd ../hello-project-feature
dclaude ln        # uses the hello-project profile, without a new login
```

- `dclaude ln <dir>` names the profile to use.
- A worktree inside the project, such as Claude Code's `.claude/worktrees/<name>`, already uses the project's profile.

## Commands

| Command | What it does |
| --- | --- |
| [`dclaude init`](#dclaude-init) | Creates a profile in the current directory and logs in |
| [`dclaude login`](#dclaude-login) | Logs in again with the current profile |
| [`dclaude ln`](#dclaude-ln) | Uses an existing profile here, without a new login |
| [`dclaude deinit`](#dclaude-deinit) | Logs out and removes the profile |
| [`dclaude info`](#dclaude-info) | Shows the account, usage and activation state |
| `dclaude version` | Prints the version |
| [`dclaude-code`](#dclaude-code) | Runs `claude` with the current directory's profile, for apps that start it without a shell |

Output is colored on a terminal. Set `NO_COLOR=1` to turn colors off, or `CLICOLOR_FORCE=1` to keep them through a pipe.

### dclaude init

```text
dclaude init [--isolated | --share-history | --no-share-history] [login options]
```

Creates a profile in the current directory and logs in.

1. Creates `.claude_config` and the dclaude block in `.envrc`.
2. Asks whether to share `~/.claude`, then whether to share the conversation history too, after telling what that costs. Both default to yes.
3. Runs `direnv allow` if the `.envrc` holds nothing else.
4. Runs `claude auth login`.
5. Says what is still needed: approving the `.envrc`, or direnv's shell hook, which it offers to add.

| Option | Effect |
| --- | --- |
| `--isolated` | Shares nothing from `~/.claude`, without asking |
| `--share-history` | Also shares the conversation history, without asking |
| `--no-share-history` | Shares `~/.claude` but keeps the conversation history in the profile, without asking |
| `--console`, `--sso`, `--email <address>` | Passed to `claude auth login` |

### dclaude login

```text
dclaude login [login options]
```

Logs in again with the current profile. Before that, it:

- re-points the profile at the new path, if the directory was moved;
- asks once whether to share `~/.claude`, for a profile that has not chosen yet: one made before sharing existed, or while `~/.claude` did not exist. The conversation history is offered only when the profile has none of its own;
- asks before moving the profile's own conversation history aside for the one in `~/.claude` (default: no);
- links entries added to `~/.claude` since, for a sharing profile.

The login options are the same as for `init`.

### dclaude ln

```text
dclaude ln [dir]
```

Makes the current directory use the profile at or above `dir`, login included, so no new login is needed.

- In a git worktree, `dir` defaults to the same place in the main checkout.
- Writes the `.envrc` block, runs `direnv allow`, and keeps the link out of `git status`.
- Running it again restores a missing `.envrc` block.

### dclaude deinit

```text
dclaude deinit [-y] [--force]
```

Logs out, then removes:

- the profile's Keychain items;
- the `.claude_config` directory, with any conversation history kept in it (a history shared with `~/.claude` stays there, like the rest of `~/.claude`);
- the dclaude block in `.envrc`;
- the links `dclaude ln` made to this profile.

In a linked directory, it removes only that link and its `.envrc` block. The profile and its login stay.

| Option | Effect |
| --- | --- |
| `-y`, `--yes` | Skips the confirmation |
| `--force` | Proceeds even while a Claude session is using the profile |

### dclaude info

Shows the profile, its sharing, the `.envrc` and direnv state, the account, and its usage. See the example in [Quick start](#quick-start).

### dclaude-code

```text
dclaude-code [claude arguments]
```

Runs `claude` the way a terminal in the current directory would. It is a small command of its own, built alongside dclaude.

- If `direnv allow` approved the `.envrc` here, it runs `direnv exec <dir> claude …`. The other variables in that `.envrc` apply too.
- Otherwise it runs `claude` unchanged. An `.envrc` that nobody approved never counts, so a cloned repository cannot pick a profile, or the hooks that come with it.
- It reads none of the arguments. `dclaude-code --version` prints Claude Code's version.
- It replaces itself with `claude`, so the app's input, output, signals and exit code reach `claude` directly.

## Agent apps

An agent app (ADE) uses a profile when it starts Claude Code in a shell, where direnv applies it, or through `dclaude-code`. These apps have been checked:

| App | Version checked | Terminals | Chat UI and agents | Setup |
| --- | --- | --- | --- | --- |
| [Orca](https://github.com/stablyai/orca) | 1.4.225 | Work as is | Need `dclaude-code` | [docs/orca.md](docs/orca.md) |
| [Paseo](https://github.com/getpaseo/paseo) | 0.11.4 | Work as is; the Claude Code terminal profile needs `dclaude-code` | Need `dclaude-code` | [docs/paseo.md](docs/paseo.md) |

Checked on macOS with Claude Code 2.1.301. The Claude Desktop app does not use profiles; see [Known issues](#known-issues).

## How it works

### What init creates

Running `dclaude init` in `~/workspace/hello-project` creates:

```text
~/workspace/hello-project/
├── .envrc               dclaude block (any other content is kept)
└── .claude_config/      the profile, used as CLAUDE_CONFIG_DIR (mode 0700)
    ├── .gitignore       "*", so the profile never reaches git
    ├── .dclaude         marker that records the profile path and the sharing choice
    ├── settings.json -> ~/.claude/settings.json
    ├── CLAUDE.md     -> ~/.claude/CLAUDE.md
    ├── plugins/      -> ~/.claude/plugins/      (and every other shared entry)
    └── projects/     -> ~/.claude/projects/     the conversation history (if you declined: the profile's own)
```

The block in `.envrc`:

```bash
# >>> dclaude >>>
# Managed by dclaude. Do not edit this block.
if [[ -d '/Users/you/workspace/hello-project/.claude_config' ]]; then
  export CLAUDE_CONFIG_DIR='/Users/you/workspace/hello-project/.claude_config'
  unset ANTHROPIC_API_KEY ANTHROPIC_AUTH_TOKEN CLAUDE_CODE_OAUTH_TOKEN CLAUDE_CODE_OAUTH_TOKEN_FILE_DESCRIPTOR CLAUDE_CODE_API_KEY_FILE_DESCRIPTOR CLAUDE_SECURESTORAGE_CONFIG_DIR
else
  log_error 'dclaude: profile not found: /Users/you/workspace/hello-project/.claude_config (if the directory moved, run: dclaude login)'
fi
# <<< dclaude <<<
```

### Path and login

- `init` records the profile path once, with symlinks resolved, and the `.envrc` block exports it verbatim.
- On macOS, Claude Code names the Keychain item after that exact string: `Claude Code-credentials-` plus the first 8 hex digits of its SHA-256. The shell, dclaude and Claude Code therefore use the same value.
- The login stays with Claude Code: in that Keychain item on macOS, and in `.claude_config/.credentials.json` on Linux. dclaude reads the access token only to query usage, and never prints it.
- Variables such as `ANTHROPIC_API_KEY` outrank a stored login, so the block unsets them inside the directory. Other tools in that shell don't see them either.

### Sharing

With sharing on, each entry of `~/.claude` gets a symlink of the same name in the profile, except these:

| Entries | Kept in the profile because |
| --- | --- |
| credentials, `.claude.json`, `policy-limits.json`, `remote-settings.json`, `mcp-needs-auth-cache.json` | they belong to the account |
| `backups`, `sessions`, `session-env`, `state`, `bridge-spawn` | they belong to running sessions |
| `projects/`, `history.jsonl`, `stats-cache.json` | they are the conversation history; linked too when you share it (the default at init, or `--share-history`) |

- If the profile already has its own copy of an entry, dclaude moves it to `.claude_config/.dclaude-backup/` before linking. For its own `projects/` or `history.jsonl` it asks first (default: no); answering no keeps them, unlinked, and the next `dclaude login` asks again.
- A profile that shares the conversation history gets an empty `~/.claude/projects/` and `~/.claude/history.jsonl` where they are missing, as Claude Code itself would create them, so the profile never starts a history of its own. Apart from that, `~/.claude` is never changed, and `deinit` removes only the links.

### Links made by dclaude ln

- The directory's `.claude_config` becomes a symlink to the profile, and its `.envrc` block exports the profile's own path. Claude Code finds the same Keychain item, so the login, settings and conversation history are the same.
- The link is listed in the repository's `.git/info/exclude`, which every worktree shares.
- The profile's `.dclaude` marker lists the linked directories, so that `deinit` can remove them.

## Limitations

### Sharing

- **A shared conversation history brings prompts.** When `projects/` is a symlink, Claude Code asks before every auto memory read or write, and allow rules cannot approve it ([anthropics/claude-code#98044](https://github.com/anthropics/claude-code/issues/98044)). `init` says so before asking; answer no, or pass `--no-share-history`, to keep the history per profile.
- **Account-scoped settings are not shared.** MCP servers added with `claude mcp add --scope user`, and per-project approvals, live in the profile's `.claude.json`.
- **The sharing choice is fixed after init.** Changing it is on the [roadmap](#roadmap).
- **An isolated profile still sees `~/.claude/CLAUDE.md`.** Claude Code reads `CLAUDE.md` in every directory above the current one, so it finds this file as a project file and asks once per project whether to allow its `@` imports. A sharing profile reads it once, as your user file.

### Directories

- **direnv loads only the nearest `.envrc`.** A subdirectory's `.envrc` hides the profile unless it starts with `source_up`. `dclaude info` points this out.
- **Moving a directory needs a new login on macOS**, because the Keychain item name follows the path. Run `dclaude login` in the new location. For a new worktree or checkout, use `dclaude ln` instead.

### Other

- **Usage comes from an internal endpoint** (`api.anthropic.com/api/oauth/usage`). It is not a public API and may stop working; `info` still shows the account if it does.
- **Isolation is best-effort.** A few Claude Code features read `~/.claude` even when `CLAUDE_CONFIG_DIR` is set. See the [open issues](https://github.com/anthropics/claude-code/issues?q=is%3Aissue+is%3Aopen+CLAUDE_CONFIG_DIR).
- **Windows is not supported**, because direnv targets Unix-like systems.

## Known issues

- First-run screens: the first `claude` run in each profile shows the theme choice, security notes and folder trust question again.
- Theme: the theme picked on that first run is saved to `~/.claude/settings.json` and applies everywhere.
- Claude Desktop app: it runs sessions with the account signed in to the app, so per-directory accounts do not apply.

## Troubleshooting

| `dclaude info` shows | Fix |
| --- | --- |
| `shell     not active yet` | Run `direnv allow`. direnv applies it at the next prompt. |
| `shell     not active: direnv is not hooked into …` | Run the two commands `info` prints below it: the first hooks new terminals, the second this one. |
| `shell     not active: this shell has not loaded direnv's hook` | This shell started before the hook was added. Open a new terminal, or run the command `info` prints. |
| `.envrc    no dclaude block in …` or `exports a different path` | Run `dclaude login`; it writes the block again. |
| `direnv    not installed` | Install direnv (see [Installation](#installation)). |
| `direnv    not allowed yet` or `denied` | Run `direnv allow`. `init` approves the `.envrc` itself only when it holds nothing but the dclaude block. |
| `direnv    … hides this profile` | Add `source_up` at the top of the `.envrc` that `info` names. |
| `shared    not chosen yet` | Run `dclaude login`; it asks once. |
| `shared    … newer entries not linked yet` | Run `dclaude login`; it links them. |
| `shared    … keeps its own projects` | Run `dclaude login`; it asks whether to move them aside. Moved entries stay in `.claude_config/.dclaude-backup/`. |
| `account   not logged in` | Run `dclaude login`. |
| `account   not checked: the directory moved` | Run `dclaude login` in the new location. |
| `usage     token expired` | Run `claude` here once; it refreshes the token. |

## Development

```bash
git clone https://github.com/powerumc/dclaude-config.git
cd dclaude-config
go test ./...
go vet ./...
go build -o bin/dclaude ./cmd/dclaude
go build -o bin/dclaude-code ./cmd/dclaude-code
```

A build from a clone takes its version from git, so `dclaude version` shows something like `0.1.1-0.20261016065125-1acbeda08d48+dirty`. Release builds set it with `-ldflags "-X main.version=0.1.0"`.

The tests use fake `claude` and `direnv` executables and an in-memory Keychain, so they never touch your accounts.

```text
cmd/dclaude/        entry point; wires dependencies only
cmd/dclaude-code/   entry point of dclaude-code
internal/cli/       the init, login, ln, deinit and info commands
internal/code/      dclaude-code: claude in the directory's direnv environment
internal/profile/   creating, finding, sharing and re-pointing profiles
internal/envrc/     the dclaude block in .envrc
internal/claude/    running the claude CLI; reading credentials and session files
internal/keychain/  Claude Code's Keychain item names and their removal
internal/direnv/    direnv allow, status and the shell hook
internal/usage/     the usage endpoint and its display
```

### Releasing

Pushing a `v*` tag starts the [release workflow](.github/workflows/release.yml):

```bash
git tag -a v0.2.0 -m v0.2.0
git push origin v0.2.0
```

1. It runs `go vet` and `go test`.
2. GoReleaser builds both commands for macOS and Linux (amd64, arm64) and publishes them on the releases page.
3. It points the formula in [powerumc/homebrew-tap](https://github.com/powerumc/homebrew-tap) at the new tag.

- A tag with a `-`, such as `v0.2.0-rc.1`, becomes a prerelease and leaves the formula alone.
- Step 3 needs the `HOMEBREW_TAP_TOKEN` repository secret, a token that can push to `powerumc/homebrew-tap`. If the token is revoked or expires, only that step fails. Set a new one with `gh secret set HOMEBREW_TAP_TOKEN -R powerumc/dclaude-config`, which prompts for it, then re-run the failed job.
- `goreleaser release --snapshot --clean` builds the same archives locally into `dist/`, without publishing them.

## Roadmap

- `dclaude share` and `dclaude unshare`: change a profile's sharing after init.
- `dclaude info --json` for status lines and scripts.

## Related projects

- [claude-swap](https://github.com/realiti4/claude-swap) switches the global Claude Code login and can map directories to accounts from a central registry (`cswap map`, `cswap run`). Choose it if you want quota-aware automatic switching.
