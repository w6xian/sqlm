# sqlm Performance Assessment Report

## 1. Overview
This report assesses the performance of `sqlm` against the rewritten benchmark suite found in `benchmark_test.go`. Every benchmark owns a private SQLite database file created through `b.TempDir()`, so results are independent of each other.

## 2. Test Environment
* **OS**: Windows
* **Architecture**: amd64
* **CPU**: Intel(R) Core(TM) i5-9400F CPU @ 2.90GHz
* **Package**: `github.com/w6xian/sqlm`

## 3. Benchmark Results

| Benchmark Name | Iterations | Time (ns/op) | Memory (B/op) | Allocations (allocs/op) | Description |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **BenchmarkConnectReuse** | 279,738,015 | 4.05 | 0 | 0 | Fetching the ready pool from a `Db` |
| **BenchmarkInstanceLookup** | 13,001,731 | 89.78 | 144 | 2 | `NewInstance` registry lookup |
| **BenchmarkBuildSQL** | 1,826,641 | 673.1 | 806 | 14 | Statement building only |
| **BenchmarkGetCount** | 92,380 | 13,428 | 1,364 | 21 | `COUNT(*)` query |
| **BenchmarkQueryRow** | 48,327 | 24,910 | 2,177 | 51 | Single row query |
| **BenchmarkQueryMulti** | 17,601 | 71,447 | 11,259 | 414 | 20-row result set |
| **BenchmarkScanStruct** | 13,411 | 89,322 | 16,042 | 478 | Scanning 20 rows into structs |
| **BenchmarkQuerySprintfWhere** | 52,837 | 22,127 | 1,490 | 28 | Legacy `Where("age = %d", v)` |
| **BenchmarkQueryMapFilters** | 51,844 | 23,044 | 1,587 | 33 | `AndFilters(map[string]any{...})` |
| **BenchmarkInsertSingle** | 993 | 1,210,116 | 1,356 | 26 | Single row insert (fsync included) |
| **BenchmarkInsertBatch** | 837 | 1,417,385 | 25,288 | 48 | 50-row batch insert |
| **BenchmarkUpdateRow** | 1,243 | 1,063,612 | 1,705 | 33 | Single row update |
| **BenchmarkTransaction** | 1,160 | 1,092,734 | 1,607 | 35 | Full transaction commit |

## 4. Analysis

### 4.1 Connection Reuse
`Connect` returns a new handle bound to the existing `*sql.DB` as soon as the pool is marked connected, without issuing another `PingContext`.

* Earlier implementation pinged on every `NewInstance`: **2,987 ns/op**.
* Now the same path costs **89.78 ns/op** (~33x faster), and grabbing the pool directly is **4.05 ns/op with zero allocations**.
* Connectivity errors are no longer probed eagerly; they surface on the executed statement and can still be probed explicitly with `Ping()`.

### 4.2 Query Construction Styles
1. **Map filters**: `AndFilters(map[string]any{...})` - 23,044 ns/op.
2. **Legacy sprintf**: `Where("age = %d", val)` - 22,127 ns/op.

**Conclusion**: the two styles are equivalent (the gap is below 5%, i.e. noise). Prefer **`AndFilters`**, which escapes values and blocks injection; keep the formatting helpers for raw SQL fragments only.

### 4.3 Writes and Transactions
Writes are in the millisecond range (1.0-1.4 ms) because SQLite must sync the WAL, while building the statement costs 673 ns. Therefore:

* Use **`Inserts`** for bulk writes: one `INSERT ... VALUES (...),(...)` amortises the commit cost over all rows.
* A transaction costs about as much as a single write, so **group several writes inside `db.Action`** to pay one commit.

### 4.4 Result Set Scanning
`ScanStruct` costs ~18 µs and 64 extra allocations versus keeping the raw `Rows`. Scanning relies on reflection and json tags, with a per-row index map reused across rows to locate columns once per query:

* Select only the columns you need (`Select("id,name")`) to cut reflection and copying.
* Skip entity mapping entirely with `QueryMulti` + `Row.Get()` when that is enough.

## 5. Conclusion
* Reusing a ready pool costs no round trip: instance acquisition dropped to the hundred-nanosecond range.
* Both query construction styles perform alike, so choose on **safety** (`AndFilters`), not speed.
* Writes are bound by the storage engine: batch inserts and transactions are the two levers that matter.

## 6. Reproducing
```bash
go test -run XXX -bench . -benchmem ./...
```
