# shutdown-drain

优雅关服：连接排空、超时强制踢、重复请求幂等，只用 Go 标准库。

```
go test ./...
go run ./cmd/shutdown-drain --sample timeout
go run ./cmd/shutdown-drain --sample idem
go run ./cmd/shutdown-drain --sample dup
```

## 口径（README 为准）

- **宽限期**：`Begin` 之后要等到 `now - startedMs >= DrainTimeoutMs` 才能强制踢剩余连接，
  宽限期内 `Drain` 返回 0。
- **开始幂等**：已在排空中再次 `Begin` 返回 `false`，并且不重置计时。
- **关闭幂等**：`Close` 只统计真正在线的连接，重复关闭不重复计数。
- **查询代价**：`Active` 不许每次全扫连接表，`Scanned` 不随连接数线性增长。
- **规模**：单服 50 万在线连接、每秒 10 万次查询，单次摊还 O(1)，内存 O(连接数)。

## 输出契约（不改格式）

```
kicked=0 active=2
second=false
kicked=1
scanned<=3000
```

## 常量

```
DrainTimeoutMs = 30000
```

## 目录

```
drain.go                排空与踢人
cmd/shutdown-drain      命令行入口
drain_test.go           go test 用例
```
