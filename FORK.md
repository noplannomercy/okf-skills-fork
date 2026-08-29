# FORK.md — G-Core fork of `xSAVIKx/okf-skills`

## Base

| | |
|---|---|
| upstream | `https://github.com/xSAVIKx/okf-skills` (remote `upstream`, 이력 fetch 완료 — `git diff upstream/master` 즉시 사용 가능) |
| fork base commit | **`9740e89309b0a10b0e847ac533cc33a7b4513f5c`** (`master`, 2026-08-07, tag `okf-go/v0.9.0`) |
| spec 정본 | `GoogleCloudPlatform/open-knowledge-format` @ `ad30107c31c06aec8a7d5636e0d1058118604e6f` — `SPEC.md` **v0.2** |
| fork 시작일 | 2026-08-29 |

이 fork는 upstream 품질에 대한 평가가 아니다. upstream의 자체 테스트는 fork 시점 기준 **전 모듈 통과**한다. fork가 존재하는 이유는 upstream이 **의도적으로 설계·문서화한 계약**이 G-Core Knowledge Baseline의 요구(evidence/provenance/trust 보존)와 맞지 않기 때문이다. 근거는 `../OKF-RE-VALIDATION.md`.

## 모듈 경로

`github.com/xSAVIKx/okf-skills/...` 를 **당분간 유지**한다. `go.work` 가 로컬 매핑을 하므로 workspace 내부 빌드는 `go.mod` 수정 0건이다 (16/16 모듈 빌드·테스트 통과로 실측).

> 독립 설치(`go install …@v0.9.0`)와 배포를 우리가 소유하려면 그때 모듈 경로를 변경한다. 13개 `go.mod` 의 기계적 변경이며 지금 할 필요는 없다.

## 패치 (okf-go, 2파일 +48/−8)

| # | 위치 | 변경 | 복원하는 요구 |
|---|---|---|---|
| **P1** | `okf.go` `IsStale` | RFC3339 instant 수용. 파싱 실패 시 **fail-closed(stale)** | 정본 SPEC §5.5 는 `stale_after` 를 **절대 시각**으로 규정. upstream 은 `YYYY-MM-DD` 만 파싱하고 실패를 `false`(신선)로 흡수 |
| **P2** | `okf.go` `VerifiedList.UnmarshalYAML` | 예상 밖 노드 종류에 **error 반환** | 이형 `verified`(스칼라/숫자/불린)가 무오류로 사라진 뒤 다음 쓰기에서 검증 기록이 영구 삭제되는 것을 차단 |
| **P3** | `okf.go` `Frontmatter.Extra` | `yaml:",inline"` catch-all 추가 | SPEC §4.1 *"consumers SHOULD preserve unknown keys when round-tripping"* |
| **P4** | `hash.go` `MergeConcept` | fresh 가 비어 있을 때 `Verified`/`Sources`/`UsageWindow`/`Status`/`StaleAfter`/`Extra` 이월 | 구조 변경 재생성 시 사람 검증·provenance·lifecycle 이 전멸하던 것을 차단. SPEC §5.2 *"content can change without re-confirmation"* |
| **P5** | `hash.go` `ConceptStructuralHash` | `type`/`title`/`resource` 를 해시 입력에 포함 | body 가 동일한 채 자산 **신원**만 바뀌면 해시가 같아 `changed==false` → 쓰기 생략 → 문서가 옛 자산을 계속 가리키고 log·diff 에 흔적이 남지 않던 무성 훼손을 차단 |

## 회귀 게이트 — `okf-go/boundary_test.go`

13개 경계 요구를 14개 테스트 함수로 고정했다. **rebase 시 이 스위트가 판정 기준이다.**

```
go test -run TestBoundary ./okf-go/...
```

| 대상 | 결과 |
|---|---|
| upstream @`9740e89` (패치 없음) | **7개 함수 FAIL** (단언 10건 실패) |
| 이 fork | **14개 함수 전부 PASS** |

게이트가 실제로 반증력이 있음을 위와 같이 확인했다. 통과만 하고 실패하지 않는 스위트는 게이트가 아니다.

## 업그레이드(마이그레이션) 주의

**P4 를 P5 보다 먼저 적용해야 한다.** P5 는 해시 정의를 바꾸므로 기존 번들의 모든 `content_hash` 가 무효화되고, 업그레이드 후 첫 `produce` 에서 전 개념이 "구조 변경"으로 판정되어 1회 전면 재작성이 일어난다. P4 가 없는 상태에서 이 churn 이 발생하면 사람 검증·provenance 가 그 1회에 전멸한다.

실측(개념 5개, 소스 무변경):

| 실행 | 결과 |
|---|---|
| 업그레이드 1회차 | 5/5 재작성 |
| 2회차 | 5/5 `Unchanged, preserved` |
| 3회차 | 5/5 `Unchanged, preserved` |
| churn 이후 `human:` 검증 | **5/5 보존** |

부수 비용: `enriched_against` 도 전 개념에서 불일치가 되어 **1회 전면 재보강 후보화**된다(토큰 비용, 데이터 훼손 아님).

## fork 정책 결정

- **커밋된 connector 바이너리를 제거했다.** upstream 은 `skills/okf-{csv,lint,viz,graphql,mongodb,openapi}/` 에 ELF 리눅스 실행 파일을 커밋해 두었다. 패치된 소스와 불일치하므로 우리 fork 에서는 적극적으로 해롭다. `.gitignore` 로 재유입을 막았다. 작업 트리 182MB → **2.5MB** (저장소 전체는 upstream 이력 팩 포함 약 60MB).
- 그 외 upstream 파일은 손대지 않았다.

## 이 fork 가 해결하지 **않는** 것

라이브러리 계층 밖의 결함은 그대로 남아 있다. 특히 `skills/okf-enrich/SKILL.md`:

- **§4c 자기서명** — 설명을 작성한 바로 그 actor 가 `verified: process:<agent>` 를 찍어 신뢰 등급을 `unverified` → `machine-confirmed` 로 올린다. 정본 SPEC §5.2 는 *"who wrote a concept need not be who confirmed it"* 로 두 축의 분리를 명시한다. (P4 덕분에 "사람 검증이 지워진 뒤 기계 서명이 얹히는" 복합 시나리오의 **파괴 단계는 차단**되었으나, 자기서명 자체는 남아 있다.)
- 근거 부족 시 **보강 보류 경로 부재**, 충돌 evidence **처리 절차 부재**, 이전 추론 **철회 규칙 부재**
- `verified` append 에 중복 제거 규칙 없음 (tags 에는 있음)

또한 `okf-viz coverage` 는 **완성도 계수기이지 정확성 검사가 아니다** — 쓰레기 설명·중복 설명·미등록 connector placeholder 를 모두 100% enriched 로 계수한다. 신뢰 무결성 게이트로 사용할 수 없다.

상세는 `../OKF-RE-VALIDATION.md` 부록 B·C.

## upstream 추적

```
git fetch upstream
git diff upstream/master --stat -- okf-go/          # 우리 divergence
go test -run TestBoundary ./okf-go/...              # 병합 후 반드시 통과해야 함
```

상류 spec 은 **버전 번호 없이 제자리 변경된 전례**가 있다(GCP `3dc3029`, v0.2 → ISO 8601 datetime, 2026-08-21). spec 정본 repo 도 함께 추적할 것.
