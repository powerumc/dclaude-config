# Paseo

[English](paseo.md) | **한국어**

Paseo 0.11.4, Claude Code 2.1.301(macOS)에서 확인했습니다.

- 일반 터미널: 설정 없이 프로필이 적용됩니다.
- Claude 에이전트, **Claude Code** 터미널 프로필: `dclaude-code`를 지정해야 프로필이 적용됩니다.

## 설정

Paseo 앱에는 이 설정 화면이 없어서 `~/.paseo/config.json`을 직접 고칩니다.

1. `dclaude-code` 실행 파일의 전체 경로를 확인합니다.

   ```bash
   command -v dclaude-code    # 예: /Users/you/go/bin/dclaude-code
   ```

2. `~/.paseo/config.json`에서 아래 두 `command` 값을 위 경로로 바꿉니다. 다른 값은 그대로 둡니다.

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

3. `paseo reload`를 실행합니다. `paseo provider diagnostic claude`에 `Command source: override`가 나오면 적용된 것입니다.

되돌리려면 `agents.providers.claude.command`를 지우고 터미널 프로필의 `command`를 `claude`로 되돌린 뒤 `paseo reload`를 실행합니다.

## 알려진 문제

- **Claude Code** 터미널 탭 이름이 `dclaude-code`로 보입니다.
