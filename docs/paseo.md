# Paseo

**English** | [한국어](paseo.ko.md)

Checked with Paseo 0.11.4 and Claude Code 2.1.301 on macOS.

- Plain terminals: use the profile without setup.
- Claude agents and the **Claude Code** terminal profile: need `dclaude-code` to use the profile.

## Setup

The Paseo app has no screen for this, so edit `~/.paseo/config.json`.

1. Find the full path of the `dclaude-code` executable.

   ```bash
   command -v dclaude-code    # e.g. /Users/you/go/bin/dclaude-code
   ```

2. In `~/.paseo/config.json`, set the two `command` values below to that path. Keep everything else.

   ```json
   {
     "agents": {
       "providers": {
         "claude": { "command": ["/Users/you/go/bin/dclaude-code"] }
       }
     },
     "daemon": {
       "terminalProfiles": [
         { "id": "claude", "name": "Claude Code", "command": "/Users/you/go/bin/dclaude-code", "args": ["{{{prompt}}}"], "icon": "claude" }
       ]
     }
   }
   ```

3. Run `paseo reload`. `paseo provider diagnostic claude` then shows `Command source: override`.

To undo, delete `agents.providers.claude.command`, set the terminal profile's `command` back to `claude`, and run `paseo reload`.

## Known issues

- The **Claude Code** terminal tab is titled `dclaude-code`.
