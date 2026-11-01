0
# tidepool

Tide data service.

## n-0aaaaa  Tide import

task: TP-03 · repo: server · agent: builder (m-1) · at: 2026-10-28T09:00:00Z · status: done

### change

- `e-0aaaa1` Wrote the tide importer.

### decision

- `e-0aaaa2` [user] Store heights in millimetres.  (superseded by n-0ddddd)

### open

- `e-0aaaa3` Which station is the reference?  (resolved by a-0ccccc)

### note

- `e-0aaaa4` Data arrives hourly.

none: helper

impacts: client

commits: abc1234

#### addendum a-0ccccc · builder (m-1) · 2026-10-30T08:15:00Z · status: done

- `e-0cccc1` [agent] Station Brest is the reference.
- resolves: e-0aaaa3

## n-0bbbbb  Chart for tides

task: TP-03.1 · repo: client · agent: reviewer (m-2) · at: 2026-10-29T10:30:00Z · status: done

### change

- `e-0bbbb1` Drew the tide chart.

### open

- `e-0bbbb2` Colour scheme for spring tides?

### note

- `e-0bbbb3` Uses the importer output.

none: decision, helper

