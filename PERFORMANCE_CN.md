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

## 7. SQL 拼装路径优化（第二轮）

`table.go` 里"拼装语句"这一段原来每一步都留下中间对象：列名切片、占位符切片、
`strings.Join`、`fmt.Sprintf`、`strings.Split`……一次 `Insert` 要 24 次分配，
50 行批量写在 PostgreSQL 下要 671 次分配。改成**一个 `strings.Builder` 直接写**，
并把能提前算的（长度预估、是否需要转义）先算掉。

新增基准位于 `benchmark_sql_test.go`，用假连接（`recConn`）只跑拼装、不含数据库往返；
`BenchmarkNoiseFloor` 用来量机器噪声下限。数据取 `-count=5 -cpu=1` 的最小值：

| 基准 | 旧 ns/op | 新 ns/op | 变化 | B/op | allocs/op |
| :--- | ---: | ---: | ---: | :--- | :--- |
| AssembleSelectSimple | 531 | 442 | -17% | 701 → 693 | 8 → 6 |
| AssembleSelectFull | 1602 | 1445 | -10% | 1811 → 1781 | 31 → 27 |
| AssembleWhereFilters | 2779 | 2469 | -11% | 1881 → 1697 | 39 → 35 |
| AssembleInsert/mysql | 1540 | 859 | -44% | 1176 → 872 | 24 → 10 |
| AssembleInsert/sqlite | 1473 | 855 | -42% | 1176 → 872 | 24 → 10 |
| AssembleInsert/postgres | 1739 | 939 | -46% | 1208 → 872 | 30 → 10 |
| AssembleInserts/mysql（50 行） | 10252 | 2298 | -78% | 14400 → 7400 | 170 → 7 |
| AssembleInserts/sqlite | 10527 | 2302 | -78% | 14400 → 7400 | 170 → 7 |
| AssembleInserts/postgres | 23634 | 9442 | -60% | 19504 → 7400 | 671 → 7 |
| AssembleUpdate/mysql | 1656 | 1071 | -35% | 1113 → 976 | 24 → 13 |
| AssembleUpdate/sqlite | 1534 | 1059 | -31% | 1088 → 976 | 22 → 13 |
| AssembleUpdate/postgres | 1529 | 1054 | -31% | 1088 → 976 | 22 → 13 |
| BenchmarkBuildSQL（既有基准） | 827 | 693 | -16% | 793 → 786 | 14 → 11 |

* **噪声下限**：`BenchmarkNoiseFloor` 为 0.54 ~ 0.71 ns/op；但本机同参数重跑的波动可达
  ±40%（见 `InsertSingle`：1.1 ms ~ 3.8 ms），所以 **ns/op 变化小于约 10% 的按噪声处理**；
  `B/op` 与 `allocs/op` 是确定性指标，本轮全部下降。
* **端到端路径无回归**：`QueryRow` 28496 → 26080（-8%，噪声内），`InsertSingle` -2%，
  `InsertBatch` +3%，`UpdateRow` +7%，均由 SQLite 磁盘往返主导；分配次数分别
  51 → 48、28 → 19、196 → 33、33 → 25。

拼装路径的行为保持不变，由 `table_assembly_test.go` 里的黄金用例逐字节锁定（三种协议
的 insert / 批量 insert / update / delete / select 文本）。复现：

```bash
go test -run='^$' -bench='BenchmarkAssemble|BenchmarkNoiseFloor|BenchmarkBuildSQL' -benchmem -count=5 -cpu=1 .
```

### 7.1 顺带修掉的三个缺陷

| 缺陷 | 复现 | 修复 |
| :--- | :--- | :--- |
| `Inserts` 就地改写调用方的 `columns`：换协议复用同一切片会生成 `` "`name`" `` 这种坏语句 | `TestInsertsLeavesCallerColumnsUntouched` | 列名直接写进 builder，不改入参 |
| `Set("discount = '50%'")` 无参数时仍跑 `Sprintf`，被写成 `50%!'(MISSING)` | `TestSetWithoutArgsKeepsFormatVerbs` | 只有传了参数才格式化 |
| SQLite / PostgreSQL 下 NUL 字节原样进入字面量（sqlite 截断语句、postgres 报 invalid byte sequence） | `TestEscapeHandlesNULPerProtocol` | 标准协议丢弃 NUL，MySQL 仍写成 `\0` |
