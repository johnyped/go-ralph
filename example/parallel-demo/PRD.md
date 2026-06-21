# parallel-demo

A minimal Go calculator project used to demonstrate go-ralph sequential and parallel execution modes.

## Goal

Four issues split across two parallel paths:

- **Path A**: 001 (scaffold) → 002 (functions) — sequential within path
- **Path B**: 003 (tests) — runs in parallel with path A once 001 is done
- **Path C**: 004 (CLI) — depends on 002 and 003 completing

With `max_parallel: 2`, paths A and B run concurrently.
