# dclaude

[English](README.md) | **한국어**

디렉토리별로 Claude Code 계정을 따로 쓰게 해 주는 도구입니다. 어떤 디렉토리에서 `dclaude init`을 실행하면, 그 디렉토리와 하위에서 시작한 Claude Code 세션은 그 디렉토리 전용 로그인을 씁니다.

- **따로 쓰는 것:** 로그인, 계정 상태, 대화 기록
- **함께 쓰는 것:** 전역 설정, `CLAUDE.md`, 플러그인, 스킬, hooks
- **그 밖의 곳:** 지금처럼 전역 계정

> **상태:** 프로토타입입니다. 명령과 파일 형식이 바뀔 수 있습니다.

## 목차

- [왜 필요한가](#왜-필요한가)
- [기능](#기능)
- [요구 사항](#요구-사항)
- [설치](#설치)
- [빠른 시작](#빠른-시작)
- [명령](#명령)
- [에이전트 앱](#에이전트-앱)
- [동작 방식](#동작-방식)
- [제한 사항](#제한-사항)
- [알려진 문제](#알려진-문제)
- [문제 해결](#문제-해결)
- [개발](#개발)
- [로드맵](#로드맵)
- [관련 프로젝트](#관련-프로젝트)

## 왜 필요한가

**디렉토리마다 Claude Code 계정을 분리하거나 디렉토리에 특정 계정을 지정해서 쓰기 위해서입니다.**

## 기능

- **디렉토리 단위 로그인.** 현재 디렉토리나 그 위에서 가장 가까운 `.claude_config`가 계정을 정합니다.
- **설정은 그대로, 계정만 따로.** 프로필이 `~/.claude`를 링크하므로 설정, `CLAUDE.md`, 플러그인, 스킬, hooks가 같습니다.
- **토큰을 복사하지 않습니다.** 프로필마다 `claude auth login`으로 로그인합니다. dclaude는 인증 정보를 쓰거나 갱신하지 않습니다.
- **워크트리는 다시 로그인하지 않습니다.** `dclaude ln`으로 git 워크트리나 다른 디렉토리를 기존 프로필에 연결합니다.
- **에이전트 앱에서도 됩니다.** 셸 없이 `claude`를 실행하는 앱에서는 `dclaude-code`를 대신 쓰면 됩니다. [지금까지 확인한 앱](#에이전트-앱)을 참고하세요.
- **안전한 `.envrc` 처리.** dclaude 구간만 고치고, dclaude가 쓰지 않은 내용은 승인하지 않습니다.
- **깔끔한 제거.** `deinit`은 로그아웃한 뒤 Keychain 항목, 프로필, `.envrc` 구간, 프로필에 연결된 링크를 지웁니다.
- **한눈에 보는 상태.** `info`는 계정, 5시간·7일 사용량, 지금 셸에 프로필이 적용됐는지를 보여 줍니다.

## 요구 사항

- macOS 또는 Linux
- `PATH`에 있는 [Claude Code](https://code.claude.com/docs) CLI(`claude`)
- 셸에 연결된 [direnv](https://direnv.net). direnv가 없으면 `init`과 `login`은 멈춥니다.
- `go install`로 설치하거나 저장소를 받아서 빌드할 때만 Go 1.26.2 이상

## 설치

1. dclaude를 설치합니다. Orca처럼 셸 없이 `claude`를 실행하는 앱에서 쓰려면 `dclaude-code`도 함께 설치합니다.

   Homebrew로 설치하면 두 명령과 direnv가 함께 설치됩니다.

   ```bash
   brew install powerumc/tap/dclaude
   ```

   Homebrew를 쓰지 않는다면 먼저 사용하는 패키지 관리자로 [direnv를 설치](https://direnv.net/docs/installation.html)한 뒤, 아래 방법 중 하나로 설치합니다.

   - **미리 빌드한 바이너리**: [릴리스 페이지](https://github.com/powerumc/dclaude-config/releases)에 macOS·Linux(amd64, arm64)용이 있습니다. 시스템에 맞는 압축 파일을 풀고 두 명령을 `PATH`에 있는 디렉토리에 둡니다.

     ```bash
     tar xzf dclaude_*_darwin_arm64.tar.gz
     mkdir -p ~/.local/bin && mv dclaude dclaude-code ~/.local/bin/
     ```

     macOS가 개발자를 확인할 수 없다며 실행을 막으면, 브라우저가 붙인 격리 속성을 지웁니다: `xattr -d com.apple.quarantine ~/.local/bin/dclaude ~/.local/bin/dclaude-code`

   - **Go**:

     ```bash
     go install github.com/powerumc/dclaude-config/cmd/dclaude@latest
     go install github.com/powerumc/dclaude-config/cmd/dclaude-code@latest
     ```

   저장소를 받아서 빌드하는 방법은 [개발](#개발)을 참고하세요.

2. direnv를 셸에 연결합니다. zsh, bash, fish, tcsh라면 `dclaude init`이 대신 추가할지 묻습니다. 직접 하려면 zsh에서는 이렇게 합니다.

   ```bash
   echo 'eval "$(direnv hook zsh)"' >> ~/.zshrc   # 새 터미널부터 적용
   eval "$(direnv hook zsh)"                      # 지금 터미널에 바로 적용
   ```

## 빠른 시작

### 프로젝트에 프로필 만들기

```bash
cd ~/workspace/hello-project
dclaude init      # 프로필을 만들고 ~/.claude 공유 여부를 물은 뒤 로그인합니다
claude            # hello-project 계정으로 실행됩니다
dclaude info      # 이 디렉토리의 계정과 사용량을 보여 줍니다
```

프로필 안에서 실행한 `dclaude info`:

```text
profile   /Users/you/workspace/hello-project/.claude_config
shared    ~/.claude: everything but the login, account state and conversation history
.envrc    /Users/you/workspace/hello-project/.envrc
direnv    allowed
shell     active
account   you@example.com, Example Org, max (claude.ai)
storage   Keychain: Claude Code-credentials-1a2b3c4d
usage     5h 22% (resets 10/15 17:10, in 1h 47m)
          7d 14% (resets 10/20 11:00, in 4d 19h)
```

프로필 밖에서 실행하면 전역 계정을 보여 줍니다.

### 워크트리와 다른 디렉토리

```bash
git worktree add ../hello-project-feature
cd ../hello-project-feature
dclaude ln        # 다시 로그인하지 않고 hello-project 프로필을 씁니다
```

- `dclaude ln <디렉토리>`로 쓸 프로필을 직접 지정할 수 있습니다.
- Claude Code의 `.claude/worktrees/<이름>`처럼 프로젝트 안에 만든 워크트리는 이미 프로젝트의 프로필을 씁니다.

## 명령

| 명령 | 하는 일 |
| --- | --- |
| [`dclaude init`](#dclaude-init) | 현재 디렉토리에 프로필을 만들고 로그인합니다 |
| [`dclaude login`](#dclaude-login) | 현재 프로필로 다시 로그인합니다 |
| [`dclaude ln`](#dclaude-ln) | 기존 프로필을 여기서도 씁니다. 다시 로그인하지 않습니다 |
| [`dclaude deinit`](#dclaude-deinit) | 로그아웃하고 프로필을 지웁니다 |
| [`dclaude info`](#dclaude-info) | 계정, 사용량, 적용 상태를 보여 줍니다 |
| `dclaude version` | 버전을 출력합니다 |
| [`dclaude-code`](#dclaude-code) | 현재 디렉토리의 프로필로 `claude`를 실행합니다. 셸 없이 `claude`를 실행하는 앱에 지정합니다 |

터미널에서는 출력에 색을 입힙니다. 색을 끄려면 `NO_COLOR=1`을 설정하세요. 파이프로 넘길 때도 색을 유지하려면 `CLICOLOR_FORCE=1`을 설정합니다.

### dclaude init

```text
dclaude init [--isolated | --share-history] [로그인 옵션]
```

현재 디렉토리에 프로필을 만들고 로그인합니다.

1. `.claude_config`와 `.envrc`의 dclaude 구간을 만듭니다.
2. `~/.claude`를 공유할지 묻습니다. 기본값은 공유입니다.
3. `.envrc`에 다른 내용이 없으면 `direnv allow`를 실행합니다.
4. `claude auth login`을 실행합니다.
5. `.envrc` 승인이나 셸 hook처럼 아직 필요한 일을 알려 줍니다. hook은 대신 추가할지 묻습니다.

| 옵션 | 효과 |
| --- | --- |
| `--isolated` | 묻지 않고, `~/.claude`를 공유하지 않습니다 |
| `--share-history` | 묻지 않고, 대화 기록까지 공유합니다 |
| `--console`, `--sso`, `--email <주소>` | `claude auth login`에 그대로 넘깁니다 |

### dclaude login

```text
dclaude login [로그인 옵션]
```

현재 프로필로 다시 로그인합니다. 그 전에 다음을 먼저 처리합니다.

- 디렉토리를 옮겼다면 프로필 경로를 새 위치로 고칩니다.
- 공유 기능이 생기기 전에 만든 프로필이면 `~/.claude`를 공유할지 한 번 묻습니다.
- 공유 중인 프로필이면 그사이 `~/.claude`에 새로 생긴 항목을 링크합니다.

로그인 옵션은 `init`과 같습니다.

### dclaude ln

```text
dclaude ln [디렉토리]
```

현재 디렉토리가 `디렉토리`나 그 위에 있는 프로필을 로그인까지 그대로 씁니다. 다시 로그인할 필요가 없습니다.

- git 워크트리 안에서 디렉토리를 생략하면 원래 체크아웃의 같은 위치를 씁니다.
- `.envrc` 구간을 쓰고 `direnv allow`를 실행합니다. 링크는 `git status`에 나오지 않게 합니다.
- 다시 실행하면 빠진 `.envrc` 구간을 되살립니다.

### dclaude deinit

```text
dclaude deinit [-y] [--force]
```

로그아웃한 뒤 다음을 지웁니다.

- 프로필의 Keychain 항목
- `.claude_config` 디렉토리와 그 안의 대화 기록 (공유된 `~/.claude`는 그대로 둡니다)
- `.envrc`의 dclaude 구간
- `dclaude ln`으로 이 프로필에 연결한 링크

연결된 디렉토리에서 실행하면 그 링크와 `.envrc` 구간만 지웁니다. 프로필과 로그인은 그대로 둡니다.

| 옵션 | 효과 |
| --- | --- |
| `-y`, `--yes` | 확인을 건너뜁니다 |
| `--force` | 이 프로필을 쓰는 Claude 세션이 실행 중이어도 진행합니다 |

### dclaude info

프로필, 공유 방식, `.envrc`와 direnv 상태, 계정, 사용량을 보여 줍니다. 예시는 [빠른 시작](#빠른-시작)에 있습니다.

### dclaude-code

```text
dclaude-code [claude 인자]
```

현재 디렉토리에서 연 터미널과 같은 환경으로 `claude`를 실행합니다. dclaude와 함께 빌드하는 작은 별도 명령입니다.

- 여기 있는 `.envrc`를 `direnv allow`로 승인했다면 `direnv exec <디렉토리> claude …`로 실행합니다. 그 `.envrc`의 다른 변수도 함께 적용됩니다.
- 그렇지 않으면 `claude`를 그대로 실행합니다. 승인하지 않은 `.envrc`는 무시하므로, 받아 온 저장소가 프로필을, 그리고 거기 딸린 hooks를 마음대로 고를 수 없습니다.
- 인자는 하나도 해석하지 않고 그대로 넘깁니다. `dclaude-code --version`은 Claude Code 버전을 출력합니다.
- 실행되면 자신을 `claude`로 바꿉니다. 그래서 앱의 입력, 출력, 시그널, 종료 코드가 `claude`와 바로 오갑니다.

## 에이전트 앱

에이전트 앱(ADE)은 Claude Code를 셸 안에서 실행하거나 `dclaude-code`를 거쳐 실행할 때 프로필을 씁니다. 셸 안에서는 direnv가 프로필을 적용합니다. 지금까지 확인한 앱은 다음과 같습니다.

| 앱 | 확인한 버전 | 터미널 | 채팅 UI·에이전트 | 설정 방법 |
| --- | --- | --- | --- | --- |
| [Orca](https://github.com/stablyai/orca) | 1.4.225 | 그대로 됩니다 | `dclaude-code` 필요 | [docs/orca.ko.md](docs/orca.ko.md) |
| [Paseo](https://github.com/getpaseo/paseo) | 0.11.4 | 그대로 됩니다. Claude Code 터미널 프로필은 `dclaude-code` 필요 | `dclaude-code` 필요 | [docs/paseo.ko.md](docs/paseo.ko.md) |

macOS에서 Claude Code 2.1.301로 확인했습니다. Claude Desktop 앱은 프로필을 쓰지 않습니다. [알려진 문제](#알려진-문제)를 참고하세요.

## 동작 방식

### init이 만드는 것

`~/workspace/hello-project`에서 `dclaude init`을 실행하면 다음이 생깁니다.

```text
~/workspace/hello-project/
├── .envrc               dclaude 구간 (다른 내용은 그대로 둠)
└── .claude_config/      CLAUDE_CONFIG_DIR로 쓰는 프로필 (권한 0700)
    ├── .gitignore       "*" 한 줄: 프로필이 git에 올라가지 않게 함
    ├── .dclaude         프로필 경로와 공유 방식을 기록한 표시 파일
    ├── settings.json -> ~/.claude/settings.json
    ├── CLAUDE.md     -> ~/.claude/CLAUDE.md
    ├── plugins/      -> ~/.claude/plugins/      (공유하는 다른 항목도 같은 방식)
    └── projects/        Claude Code가 만드는 이 프로필의 대화 기록
```

`.envrc`에 들어가는 구간:

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

### 경로와 로그인

- `init`은 심볼릭 링크까지 풀어서 프로필 경로를 한 번 기록하고, `.envrc` 구간은 그 문자열을 그대로 내보냅니다.
- macOS에서 Claude Code는 바로 이 문자열로 Keychain 항목 이름을 정합니다. `Claude Code-credentials-` 뒤에 SHA-256 앞 8자리 16진수를 붙인 형태입니다. 그래서 셸, dclaude, Claude Code가 모두 같은 값을 씁니다.
- 로그인은 Claude Code가 관리합니다. macOS에서는 그 Keychain 항목에, Linux에서는 `.claude_config/.credentials.json`에 저장됩니다. dclaude는 사용량을 조회할 때만 access token을 읽고, 화면에는 출력하지 않습니다.
- `ANTHROPIC_API_KEY` 같은 변수는 저장된 로그인보다 우선합니다. 그래서 이 구간이 디렉토리 안에서 그 변수를 지웁니다. 그 셸에서 실행하는 다른 도구도 이 변수를 보지 못합니다.

### 공유

공유를 켜면 `~/.claude`의 항목마다 같은 이름의 심볼릭 링크를 프로필에 만듭니다. 다만 다음은 프로필에 따로 둡니다.

| 항목 | 따로 두는 이유 |
| --- | --- |
| 인증 정보, `.claude.json`, `policy-limits.json`, `remote-settings.json`, `mcp-needs-auth-cache.json` | 계정에 딸린 것 |
| `backups`, `sessions`, `session-env`, `state`, `bridge-spawn` | 실행 중인 세션에 딸린 것 |
| `projects/`, `history.jsonl`, `stats-cache.json` | 대화 기록. `--share-history`를 주면 이것도 링크합니다 |

- 프로필에 같은 이름의 항목이 이미 있으면 링크하기 전에 `.claude_config/.dclaude-backup/`으로 옮깁니다.
- `~/.claude` 자체는 바꾸지 않습니다. `deinit`도 링크만 지웁니다.

### dclaude ln이 만드는 링크

- 연결한 디렉토리의 `.claude_config`는 프로필로 가는 심볼릭 링크가 되고, `.envrc` 구간은 프로필의 원래 경로를 그대로 내보냅니다. Claude Code가 같은 Keychain 항목을 찾으므로 로그인, 설정, 대화 기록이 같습니다.
- 링크는 모든 워크트리가 함께 쓰는 `.git/info/exclude`에 추가합니다.
- 연결한 디렉토리 목록은 프로필의 `.dclaude` 표시 파일에 남겨 두었다가 `deinit` 때 함께 정리합니다.

## 제한 사항

### 공유

- **대화 기록은 기본적으로 프로필마다 따로입니다.** `projects/`가 심볼릭 링크이면 Claude Code가 auto memory를 읽고 쓸 때마다 확인을 받고, 허용 규칙으로도 넘길 수 없습니다([anthropics/claude-code#98044](https://github.com/anthropics/claude-code/issues/98044)). `--share-history`를 주면 이 확인 창을 감수하고 함께 씁니다.
- **계정에 딸린 설정은 공유되지 않습니다.** `claude mcp add --scope user`로 추가한 MCP 서버와 프로젝트별 승인 내역은 프로필의 `.claude.json`에 저장됩니다.
- **공유 방식은 init 뒤에 바꿀 수 없습니다.** 나중에 바꾸는 기능은 [로드맵](#로드맵)에 있습니다.
- **분리한 프로필에서도 `~/.claude/CLAUDE.md`는 보입니다.** Claude Code는 현재 디렉토리부터 위쪽 모든 디렉토리의 `CLAUDE.md`를 읽습니다. 그래서 이 파일을 프로젝트 파일로 읽고, `@`로 가져오는 파일을 허용할지 프로젝트마다 한 번 묻습니다. 공유 중인 프로필은 이 파일을 사용자 파일로 한 번만 읽습니다.

### 디렉토리

- **direnv는 가장 가까운 `.envrc` 하나만 읽습니다.** 하위 디렉토리의 `.envrc`가 `source_up`으로 시작하지 않으면 프로필이 가려집니다. `dclaude info`가 이 경우를 알려 줍니다.
- **macOS에서는 디렉토리를 옮기면 다시 로그인해야 합니다.** Keychain 항목 이름이 경로를 따라가기 때문입니다. 새 위치에서 `dclaude login`을 실행하세요. 새 워크트리나 다른 체크아웃이라면 `dclaude ln`을 쓰면 됩니다.

### 그 밖

- **사용량은 내부 엔드포인트에서 가져옵니다.** `api.anthropic.com/api/oauth/usage`는 공개 API가 아니라서 언제든 동작하지 않을 수 있습니다. 그래도 `info`는 계정 정보를 계속 보여 줍니다.
- **격리는 완벽하지 않습니다.** `CLAUDE_CONFIG_DIR`를 설정해도 Claude Code의 일부 기능은 `~/.claude`를 읽습니다. [열린 이슈](https://github.com/anthropics/claude-code/issues?q=is%3Aissue+is%3Aopen+CLAUDE_CONFIG_DIR)를 참고하세요.
- **Windows는 지원하지 않습니다.** direnv가 Unix 계열 시스템을 대상으로 하기 때문입니다.

## 알려진 문제

- 초기 설정 화면: 프로필마다 처음 `claude`를 실행하면 테마 선택, 보안 안내, 폴더 신뢰 확인이 다시 나옵니다.
- 테마: 처음 실행할 때 고른 테마가 `~/.claude/settings.json`에 저장되어 모든 곳에 적용됩니다.
- Claude Desktop 앱: 앱에 로그인한 계정으로 세션을 실행하므로 디렉토리별 계정이 적용되지 않습니다.

## 문제 해결

| `dclaude info`에 나오는 내용 | 해결 |
| --- | --- |
| `shell     not active yet` | `direnv allow`를 실행하세요. 다음 프롬프트부터 적용됩니다. |
| `shell     not active: direnv is not hooked into …` | `info`가 아래에 보여 주는 두 줄을 실행하세요. 첫 줄은 새 터미널용, 둘째 줄은 지금 터미널용입니다. |
| `shell     not active: this shell has not loaded direnv's hook` | 지금 셸이 hook을 추가하기 전에 시작됐습니다. 새 터미널을 열거나 `info`가 보여 주는 명령을 실행하세요. |
| `.envrc    no dclaude block in …` 또는 `exports a different path` | `dclaude login`을 실행하면 구간을 다시 씁니다. |
| `direnv    not installed` | direnv를 설치하세요([설치](#설치) 참고). |
| `direnv    not allowed yet` 또는 `denied` | `direnv allow`를 실행하세요. `init`은 `.envrc`에 dclaude 구간만 있을 때만 직접 승인합니다. |
| `direnv    … hides this profile` | `info`가 가리키는 `.envrc` 맨 위에 `source_up`을 추가하세요. |
| `shared    not chosen yet` | `dclaude login`을 실행하세요. 한 번 묻습니다. |
| `shared    … newer entries not linked yet` | `dclaude login`을 실행하면 링크합니다. |
| `account   not logged in` | `dclaude login`을 실행하세요. |
| `account   not checked: the directory moved` | 새 위치에서 `dclaude login`을 실행하세요. |
| `usage     token expired` | 이 디렉토리에서 `claude`를 한 번 실행하면 토큰을 갱신합니다. |

## 개발

```bash
git clone https://github.com/powerumc/dclaude-config.git
cd dclaude-config
go test ./...
go vet ./...
go build -o bin/dclaude ./cmd/dclaude
go build -o bin/dclaude-code ./cmd/dclaude-code
```

저장소에서 빌드하면 버전을 git에서 가져오므로, `dclaude version`은 `0.1.1-0.20261016065125-1acbeda08d48+dirty` 같은 값을 보여 줍니다. 릴리스 빌드는 `-ldflags "-X main.version=0.1.0"`으로 버전을 넣습니다.

테스트는 가짜 `claude`, `direnv` 실행 파일과 메모리 안의 가짜 Keychain을 쓰므로 실제 계정을 건드리지 않습니다.

```text
cmd/dclaude/        진입점 (의존성 연결만)
cmd/dclaude-code/   dclaude-code 진입점
internal/cli/       init, login, ln, deinit, info 명령
internal/code/      dclaude-code: 디렉토리의 direnv 환경으로 claude 실행
internal/profile/   프로필 생성, 탐색, 공유, 경로 재지정
internal/envrc/     .envrc의 dclaude 구간
internal/claude/    claude CLI 실행, 인증 정보와 세션 파일 읽기
internal/keychain/  Claude Code의 Keychain 항목 이름 계산과 삭제
internal/direnv/    direnv allow, status, 셸 hook
internal/usage/     사용량 엔드포인트 조회와 표시
```

### 릴리스

`v*` 태그를 push하면 [릴리스 워크플로](.github/workflows/release.yml)가 시작됩니다.

```bash
git tag -a v0.2.0 -m v0.2.0
git push origin v0.2.0
```

1. `go vet`과 `go test`를 실행합니다.
2. GoReleaser가 두 명령을 macOS·Linux(amd64, arm64)용으로 빌드해 릴리스 페이지에 올립니다.
3. [powerumc/homebrew-tap](https://github.com/powerumc/homebrew-tap)의 formula를 새 태그로 바꿉니다.

- `v0.2.0-rc.1`처럼 `-`가 들어간 태그는 prerelease가 되고, formula는 바꾸지 않습니다.
- 3단계에는 `powerumc/homebrew-tap`에 push할 수 있는 토큰을 담은 `HOMEBREW_TAP_TOKEN` 저장소 secret이 필요합니다. 토큰이 폐기되거나 만료되면 이 단계만 실패합니다. `gh secret set HOMEBREW_TAP_TOKEN -R powerumc/dclaude-config`를 실행하면 토큰을 묻는 프롬프트가 뜨니, 새 토큰을 넣고 실패한 job을 다시 실행하세요.
- `goreleaser release --snapshot --clean`을 실행하면 같은 압축 파일을 올리지 않고 `dist/`에만 만듭니다.

## 로드맵

- `dclaude share`, `dclaude unshare`: init 뒤에 프로필의 공유 방식 바꾸기
- [anthropics/claude-code#98044](https://github.com/anthropics/claude-code/issues/98044)가 고쳐지면 대화 기록도 기본으로 공유
- 상태 표시줄과 스크립트용 `dclaude info --json`

## 관련 프로젝트

- [claude-swap](https://github.com/realiti4/claude-swap)은 Claude Code의 전역 로그인을 전환하는 도구입니다. 중앙 목록으로 디렉토리와 계정을 연결하는 기능(`cswap map`, `cswap run`)도 있습니다. 사용량에 따라 계정을 자동으로 바꾸고 싶다면 이쪽이 맞습니다.
