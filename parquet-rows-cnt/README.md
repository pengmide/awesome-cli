# parquet-rowcount

一个用 Go 写的 Parquet 行数统计小工具。它直接读取 Parquet 元数据中的行数，不会逐行扫描文件。

## 用法

```bash
go run ./cmd/parquet-rowcount -- data.parquet
go run ./cmd/parquet-rowcount -- a.parquet b.parquet c.parquet
go run ./cmd/parquet-rowcount --total-only data.parquet
```

输出格式默认是：

```text
12345	data.parquet
```

传多个文件时，会额外输出总数：

```text
100	a.parquet
200	b.parquet
300	total
```

## 构建

本项目使用纯 Go 依赖，默认可跨平台编译。

### 当前平台构建

```bash
go build -o ../bin/parquet-rowcount ./cmd/parquet-rowcount
```

### 常见平台交叉编译

```bash
./scripts/build-all.sh
```

Windows PowerShell:

```powershell
./scripts/build-all.ps1
```
