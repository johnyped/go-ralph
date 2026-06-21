# 003 — Add tests for calc package

Write unit tests covering all four functions.

## Acceptance Criteria

- [ ] `calc/calc_test.go` exists with `package calc`
- [ ] Tests cover Add, Sub, Mul, and Div (including divide-by-zero case)
- [ ] `go test ./calc/...` passes

## Summary

- All four calc functions (Add, Sub, Mul, Div) have passing unit tests
- Divide-by-zero case tested: `Div(x, 0)` returns error
- 5 tests total, all passing: TestAdd, TestSub, TestMul, TestDiv, TestDivByZero

### Files
- `calc/calc_test.go` — test file (existed, all tests verified)
- `calc/calc.go` — implementation (existed, Mul function added)
