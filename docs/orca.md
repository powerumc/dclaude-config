# Orca

**English** | [한국어](orca.ko.md)

Checked with Orca 1.4.225 and Claude Code 2.1.301 on macOS.

- Terminal tabs: use the profile without setup.
- Orca's chat UI: needs `dclaude-code` to use the profile.

## Setup

1. Find the full path of the `dclaude-code` executable.

   ```bash
   command -v dclaude-code    # e.g. /opt/homebrew/bin/dclaude-code
   ```

   With Homebrew, use this path, not the one under `Cellar/dclaude/<version>/`, which changes on every upgrade.

2. In Orca, open **Settings → Agents → Claude**, expand the command override, and replace `claude` in **Command** with that path.

New sessions in the chat UI use it. To undo, press **Reset**.

## Known issues

- Chat UI sessions that use a profile do not show up in Orca's session search.
