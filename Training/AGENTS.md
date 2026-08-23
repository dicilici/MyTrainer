# AGENTS.md

## 构建 / 代码检查 / 测试 命令

```bash
# 构建所有包
go build ./...

# 构建当前包
go build .

# 运行所有测试
go test ./...

# 运行所有测试（详细输出）
go test -v ./...

# 运行单个测试函数
go test -v -run TestFunctionName ./path/to/package

# 示例：在 config 包中运行名为 "TestConfig" 的测试
go test -v -run TestConfig ./config/

# 使用竞态检测器运行测试
go test -race ./...

# 检查常见 Go 错误
go vet ./...

# 格式化所有 Go 源文件
go fmt ./...

# 整理 go.mod
go mod tidy

# 重新生成 protobuf 代码
protoc --go_out=. --go_opt=module=train --go-grpc_out=. --go-grpc_opt=module=train sender/sender.proto
protoc --go_out=. --go_opt=module=train --go-grpc_out=. --go-grpc_opt=module=train Database/dbHandler.proto
```

**注意：** 本模块为 `train`，Go 版本 `1.25.10`。外部依赖：`google.golang.org/grpc`、`google.golang.org/protobuf`（及间接依赖）。无 Makefile / CI / linting 配置。

---

## 代码风格指南

### 1. 导入

- 顺序：标准库 → 第三方（gRPC等） → 内部模块，用空行分隔
- 每个文件一个 `import` 块
- 不使用空白导入或点导入

```go
import (
    "context"
    "fmt"
    "os"

    "google.golang.org/grpc"

    "train/config"
    "train/pkg"
)
```

### 2. 命名约定

- **导出类型/函数/变量：** 大驼峰（`Config`, `NewDefaultIniter`, `IniterRegister`）
- **未导出类型/函数/变量：** 小驼峰或全小写（`criteria`, `selectData`, `manager` 接口）
- **包名：** 小写、简短、单个单词（`config`, `pkg`, `selector`, `manager`, `sender`）
  - 例外：`Database` 包使用大写 D，尽量保持一致使用小写
- **文件名：** 大驼峰或小驼峰（`config.go`, `Initer.go`, `handleJSON.go`, `Idmanager.go`）
- **JSON 标签：** 顶层配置用 `snake_case`（`json:"train_backend_url"`），内嵌结构体用 `PascalCase`（`json:"DbName"`, `json:"Input"`）
- **构造函数：** `New<Type>(...)` 返回 `*Type`
- **接口名：** 后缀 `er` 或不带后缀（`Initer`, `Selector`, `Client`, `Manager`, `Handler`）

### 3. 格式化

- 使用制表符缩进（Go 标准）
- 提交前运行 `go fmt ./...`
- 结构体字段间不加空行
- import 块和函数定义之间加空行
- `//` 注释后加空格

### 4. 类型与结构体

- 导出结构体字段全部带 `json:"..."` 标签
- 组合优于嵌入：用命名字段组合结构体
- 基于接口的设计，偏好小而专注的接口（2-5 个方法）

```go
type Config struct {
    Name        string      `json:"name"`
    Db          Database    `json:"db"`
    Dataset     Dataset     `json:"dataset"`
    TrainConfig TrainConfig `json:"train_config"`
}
```

### 5. 函数与方法

- 方法使用**指针接收者**（`func (d *DefaultIniter) Init(...)`）
- 构造函数返回 `*Type` 指针
- 可选的构造参数使用可变参数

### 6. 错误处理

- 标准模式：`if err != nil { return err }`
- 静态错误用 `errors.New("...")`，包装错误用 `fmt.Errorf("...: %w", err)`
- 返回错误，绝不 panic
- 验证必填字段并返回描述性错误

### 7. 通用模式

- 每文件一个主要类型/主题
- 避免 `init()` 函数
- 成功打开文件后立即 `defer file.Close()`
- 读取文件优先用 `os.ReadFile` 或 `io.ReadAll`，而非固定大小缓冲区
- 日志模式：同时写入 `log.Println` 和 `file.WriteString`

### 8. 已知代码问题（修复前请确认）

| # | 位置 | 问题 |
|---|---|---|
| 1 | `selector/selector.go:4` | `Selector` 接口方法 `selectData` 未导出，导致接口在包外无法实现 |
| 2 | `Database/database.go:18-22` | `NewDatabaseConfig()` 中 `make([]*DataCriteria, 100)` 创建 nil 指针切片，`criterias[index].Field` 解引用会 panic |
| 3 | `config/Initer.go:68` | 条件表达式缺少括号，`&&` 优先级高于 `\|\|`，可能导致逻辑错误 |
| 4 | `config/Initer.go:98-108` | 硬编码 500 字节缓冲区，`Read` 后未 `reslice`，可能读到空字节 |
| 5 | `manager/manager.go:57-67` | `Load()` 实际是写操作，`Store()` 是读操作，命名与语义相反 |
| 6 | `pkg/log.go:9-12` | `string(id)` 将 int32 转为 Unicode 码点而非数字字符串，应使用 `strconv.Itoa` |
| 7 | `manager/manager.go:57` | `fmt.Sprint(..., "/manager", "txt")` 缺少点号，应为 `"/manager.txt"` |
| 8 | `Database/database.go:77-78` | `re.IsOk == false` 时分支持有 nil 的 `err` 传给 `pkg.Log` |

### 9. 项目结构

```
Training/
├── .idea/               # GoLand 配置
├── config/              # 配置类型 (Config, Database, Dataset, TrainConfig)
│   ├── config.go        # 结构体定义和初始化 (Initer 接口 + DefaultIniter)
│   └── Initer.go
├── Database/            # 数据库 gRPC 处理
│   ├── database.go      # 手写业务逻辑
│   ├── dbHandler.proto  # protobuf 定义
│   ├── dbHandler.pb.go           # protoc 生成
│   └── dbHandler_grpc.pb.go      # protoc 生成
├── manager/             # ID 管理
│   ├── Idmanager.go     # ID 池 (DefaultIdManager)
│   └── manager.go       # 键值对管理 (DefaultManager)
├── pkg/                 # 工具包
│   ├── handleJSON.go    # Analysis() — 解析 "---" 分隔的 JSON
│   ├── handleManager.go # HandlerManager() — 解析 "k:v" 键值对
│   └── log.go           # Log() / LogWithString() — 日志辅助
├── selector/            # 选择器接口和默认实现
│   └── selector.go      # Criteria, Selector, DefaultSelector
├── sender/              # 训练 gRPC 客户端
│   ├── sender.go        # 手写业务逻辑
│   ├── sender.proto     # protobuf 定义
│   ├── sender.pb.go              # protoc 生成
│   └── sender_grpc.pb.go         # protoc 生成
├── go.mod
├── go.sum
└── AGENTS.md
```

### 10. GoLand / IDE

- 本项目使用 GoLand（`.idea/` 目录存在），Go 1.25.10 toolchain 已配置
- 将 `.idea/` 加入 `.gitignore` 以保留用户特定设置

### 11. 测试（编写测试时）

- 使用 Go 标准 `testing` 包，表驱动测试
- 测试文件命名：`<源文件>_test.go`
- 使用 `t.Run("子测试名", ...)` 组织子测试
- 使用 `t.Fatal` / `t.Errorf`，不要 `panic`
- 测试放在与被测代码相同的包中
