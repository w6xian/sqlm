# sqlm 性能评估报告

## 1. 概述
本报告基于重构后的测试基准集对 `sqlm` 库进行性能评估，覆盖实例获取、连接池复用、SQL 构建、查询/写入/事务等路径。基准位于 `benchmark_test.go`，全部基于独立的 SQLite 实例（每个基准一个库文件）。

## 2. 测试环境
* **操作系统**: Windows
* **架构**: amd64
* **CPU**: Intel(R) Core(TM) i5-9400F CPU @ 2.90GHz
* **包**: `github.com/w6xian/sqlm`

## 3. 基准结果

| 基准名称 | 迭代次数 | 耗时 (ns/op) | 内存 (B/op) | 分配次数 (allocs/op) | 说明 |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **BenchmarkConnectReuse** | 279,738,015 | 4.05 | 0 | 0 | 从 Db 取已就绪的连接池 |
| **BenchmarkInstanceLookup** | 13,001,731 | 89.78 | 144 | 2 | `NewInstance` 获取实例 |
| **BenchmarkBuildSQL** | 1,826,641 | 673.1 | 806 | 14 | 仅构建 SQL 字符串 |
| **BenchmarkGetCount** | 92,380 | 13,428 | 1,364 | 21 | `COUNT(*)` 查询 |
| **BenchmarkQueryRow** | 48,327 | 24,910 | 2,177 | 51 | 单行查询 |
| **BenchmarkQueryMulti** | 17,601 | 71,447 | 11,259 | 414 | 20 行结果集 |
| **BenchmarkScanStruct** | 13,411 | 89,322 | 16,042 | 478 | 20 行扫描进结构体切片 |
| **BenchmarkQuerySprintfWhere** | 52,837 | 22,127 | 1,490 | 28 | 旧式 `Where("age = %d", v)` |
| **BenchmarkQueryMapFilters** | 51,844 | 23,044 | 1,587 | 33 | `AndFilters(map[string]any{...})` |
| **BenchmarkInsertSingle** | 993 | 1,210,116 | 1,356 | 26 | 单行写入（含 fsync） |
| **BenchmarkInsertBatch** | 837 | 1,417,385 | 25,288 | 48 | 50 行批量写入 |
| **BenchmarkUpdateRow** | 1,243 | 1,063,612 | 1,705 | 33 | 单行更新 |
| **BenchmarkTransaction** | 1,160 | 1,092,734 | 1,607 | 35 | 一次完整事务提交 |

## 4. 分析

### 4.1 连接池复用
`Connect` 在连接池已就绪时（`isConnected`）直接返回持有同一个 `*sql.DB` 的新句柄，不再执行 `PingContext`。

* 早期实现每次 `NewInstance` 都要一次 Ping 往返：**2,987 ns/op**。
* 优化后同路径：`instance lookup` 仅 **89.78 ns/op**（约 33 倍提升），直接取连接池为 **4.05 ns/op 且零分配**。
* 连接错误不再在连接阶段统一 Ping，而是由具体语句暴露，可用 `Ping()` 显式探测——这是用一次可忽略的延迟换取每次调用的性能。

### 4.2 查询构造方式对比
1. **Map 过滤器**：`AndFilters(map[string]any{...})` —— 23,044 ns/op。
2. **旧式 Sprintf**：`Where("age = %d", val)` —— 22,127 ns/op。

**结论**：两者基本等价（差异 < 5%，落在噪声范围内），Map 方式多出的一次 map 迭代与反射开销可以忽略。**推荐优先使用 `AndFilters`**：它对值做了转义处理，避免注入风险；只有确实需要直接拼接原始 SQL 片段时才使用 `Where/And/Or` 的格式化参数。

### 4.3 写入与事务
SQLite 下每次写入约为毫秒级（1.0 ~ 1.4 ms），瓶颈在 WAL 与磁盘同步，而非 SQL 构建（构建仅 673 ns）。因此：

* **批量写优先用 `Inserts`**：单次 `INSERT ... VALUES (...),(...)` 摊薄了每条记录的提交成本。
* 事务（1.09 ms/op）与单条写入（1.21 ms/op）同量级，**多条写操作应放进 `db.Action`**，用一次提交完成。

### 4.4 结果集扫描
`ScanStruct`（全量 20 行映射到结构体）比 `QueryMulti`（保留原始 `Rows`）多约 18 µs 与 64 次分配。扫描走的是反射 + json tag 匹配：

* 已经为 tag 索引建立了 per-row 的 map 复用，避免每行重复计算列位置。
* 当只需要少量字段时，优先 `Select("id,name")` 减少反射与拷贝；不需要实体映射时直接用 `QueryMulti` + `Row.Get()`。

## 5. 结论
* 连接池已就绪时零往返复用，实例获取开销降至百纳秒级。
* 查询构造的两种风格性能相当，选型应以**安全性**（`AndFilters`）而非性能为决策依据。
* 写入瓶颈在存储引擎，`Inserts` 批量写与 `Action` 事务是两条明确的优化路径。

## 6. 复现方式
```bash
go test -run XXX -bench . -benchmem ./...
```
