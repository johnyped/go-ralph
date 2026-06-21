# 002 — Add calculator functions

Implement Add, Sub, Mul, Div in the calc package.

## Acceptance Criteria

- [ ] `calc/calc.go` exports `func Add(a, b float64) float64`
- [ ] `calc/calc.go` exports `func Sub(a, b float64) float64`
- [ ] `calc/calc.go` exports `func Mul(a, b float64) float64`
- [ ] `calc/calc.go` exports `func Div(a, b float64) (float64, error)` returning error when b == 0
- [ ] `go build ./calc/...` succeeds

## Summary

- **Add**: `Add(a, b float64) float64` — returns sum of two floats
- **Sub**: `Sub(a, b float64) float64` — returns difference of two floats
- **Mul**: `Mul(a, b float64) float64` — returns product of two floats
- **Div**: `Div(a, b float64) (float64, error)` — returns quotient; returns `"division by zero"` error when b == 0

### Files modified
- `calc/calc.go` — added Add, Sub, Mul, Div functions
- `calc/calc_test.go` — added unit tests for all four functions (including divide-by-zero)
