# 세션 컨텍스트 — okf-skills fork

`xSAVIKx/okf-skills` @ `9740e89` 의 fork 다. 배경·판정·남은 작업은 별도 private 저장소
`github.com/noplannomercy/okf-series-validation` 에 있다. 이 파일은 **이 코드베이스를 건드릴 때의 규칙**만 담는다.

패치 목록과 각각의 SPEC 근거는 [`FORK.md`](FORK.md) 를 먼저 읽어라.

## 손대기 전에 알아야 할 것

**회귀 게이트가 이 fork 의 존재 이유다.**

```bash
cd okf-go && go test -run TestBoundary ./...        # 16개 함수 전부 PASS 여야 함
cd skills/okf-lint && go test -run TestPolicy ./... # trust policy gate
```

- **Producer boundary gate = 16** (`okf-go/boundary_test.go`)
- **Trust policy enforcement gate = +1** (`skills/okf-lint/policy.go`)
- 계층이 다르므로 **합산하지 않는다**

상류를 병합한 뒤 이 중 하나라도 깨지면 **fork 의 존재 이유가 회귀한 것**이다. 게이트를 고쳐서 통과시키지 말고 원인을 찾아라.

## 지켜야 할 경계

1. **SPEC conformance 와 우리 정책을 섞지 않는다.**
   `okf.LintReport.Conformance` 는 OKF v0.2 conformance 전용이다. 우리 정책 위반은 `skills/okf-lint/policy.go` 의 `PolicyFinding` 으로 가며 규칙 id 는 `policy-` 네임스페이스다. `generated.by == verified.by` 는 **SPEC 위반이 아니다.**
   policy gate 는 **opt-in**(`-policy-no-self-sign`)이고, 켜지 않으면 stdout 과 exit code 가 upstream 과 동일해야 한다.

2. **코드 수정 전에 baseline FAIL 을 먼저 재현한다.**
   통과만 하고 실패하지 않는 테스트는 게이트가 아니다. 새 경계를 닫을 때는 반드시 (a) 실패하는 fixture 를 먼저 만들고 (b) 고치고 (c) `boundary_test.go` 에 승격한다.

3. **`MergeConcept` 의 이월 로직은 컬렉션 전체 치환을 하지 않는다.**
   `Extra` 는 키 단위 병합(P4b), `Verified` 는 `(by, at)` 중복 제거 병합(P4c). **`Sources` 는 아직 all-or-nothing 이며 미해결 boundary candidate 다** — 항목 동일성 규칙을 정하기 전에 손대지 마라. 잘못 고르면 provenance 가 조용히 합쳐지거나 중복된다.

4. **`ConceptStructuralHash` 를 바꾸면 기존 번들의 모든 `content_hash` 가 무효화된다.**
   1회 전면 재작성이 발생한다. P4(이월)가 먼저 들어가 있어야 그 churn 이 비파괴적이다.

5. **upstream PR 을 열 때는 이 저장소 main 을 그대로 밀지 마라.**
   `upstream/master` 에서 새 브랜치를 따고 해당 소스 변경만 cherry-pick 한다. `FORK.md` 와 `boundary_test.go` 는 우리 요구사항이지 upstream 요구사항이 아니다.

## upstream 추적

```bash
git fetch upstream
git diff upstream/master --stat -- okf-go/   # 우리 divergence
```

상류 spec 은 **버전 번호 없이 제자리 변경된 전례**가 있다 (GCP `3dc3029`, v0.2 → ISO 8601 datetime). spec 정본 저장소도 함께 추적할 것.
