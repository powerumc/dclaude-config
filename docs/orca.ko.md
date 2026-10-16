# Orca

[English](orca.md) | **한국어**

Orca 1.4.225, Claude Code 2.1.301(macOS)에서 확인했습니다.

- 터미널 탭: 설정 없이 프로필이 적용됩니다.
- Orca 채팅 UI: `dclaude-code`를 지정해야 프로필이 적용됩니다.

## 설정

1. `dclaude-code` 실행 파일의 전체 경로를 확인합니다.

   ```bash
   command -v dclaude-code    # 예: /opt/homebrew/bin/dclaude-code
   ```

   Homebrew로 설치했다면 이 경로를 쓰세요. `Cellar/dclaude/<버전>/` 아래 경로는 업그레이드할 때마다 바뀝니다.

2. Orca **설정 → 에이전트 → Claude**에서 명령 재정의를 펼치고, **명령** 칸의 `claude`를 위 경로로 바꿉니다.

채팅 UI에서 새로 여는 세션부터 적용됩니다. 되돌리려면 **재설정**을 누릅니다.

## 알려진 문제

- 채팅 UI에서 프로필을 쓴 세션은 Orca 세션 검색에 나오지 않습니다.
