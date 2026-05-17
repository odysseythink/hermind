# Qt ↔ Go 通信架构分析报告

## 一、参考项目 (models_gen) 通信机制解析

### 1.1 核心发现：不是 gRPC over network，而是 CGO in-process 调用

经过完整源码分析，参考项目 `models_gen` **并未使用 gRPC 网络协议**，而是采用了以下架构：

```
┌─────────────────────────────────────────────────────────────┐
│                    单一进程 (hermind-desktop.exe)              │
│  ┌─────────────────────┐     CGO      ┌──────────────────┐  │
│  │     Qt/C++ 前端      │ ◄──────────► │   Go 静态库(.a)   │  │
│  │                     │  Protobuf    │                  │  │
│  │  • go_interface.cpp │   bytes      │  • main.go       │  │
│  │  • Call_Go_Func()   │              │  • //export 函数  │  │
│  │  • CGoInterface     │              │  • C.malloc      │  │
│  └─────────────────────┘              └──────────────────┘  │
└─────────────────────────────────────────────────────────────┘
```

### 1.2 数据流向详解

**C++ → Go（同步调用）**
```cpp
// 1. C++ 构造 Protobuf 请求
pbapi::PK_OPEN_DB_REQ req;
req.set_host("localhost");

// 2. 序列化后传给 Go 导出的 C 函数
pbapi::PK_OPEN_DB_RSP rsp;
Call_Go_Func(&req, &rsp, OpenDb);  // OpenDb 是 Go //export 的函数
```

```go
// 3. Go 侧反序列化、处理、再序列化返回
//export OpenDb
func OpenDb(req unsafe.Pointer, reqlen C.int, l *C.int) unsafe.Pointer {
    myreq := pbapi.PK_OPEN_DB_REQ{}
    parseReqFromCPointer(req, reqlen, &myreq)
    // ... 业务逻辑 ...
    return packRspToCPointer(&myrsp, l)  // 用 C.malloc 分配内存返回
}
```

**Go → C++（异步通知）**
```go
// Go 侧主动通知 C++
C.C_Callback(cmd, p, length)  // 调用 C 侧的 C_Callback 函数
```

```cpp
// C++ 侧注册接收器，通过 Qt 信号槽异步处理
CGoInterface::GetInstance()->RegisterHandler(cmd, receiver, SLOT(onMessage(QByteArray)));
```

### 1.3 编译方式

```bash
# Go 编译为 C 静态库（不是可执行文件！）
go build -buildmode=c-archive -o libmodels_gen.a

# 输出两个文件：
# • libmodels_gen.a   — 静态库（包含 Go runtime + 业务逻辑）
# • libmodels_gen.h   — C 头文件（包含 //export 函数的声明）
```

C++ CMake 链接这个 `.a` 文件即可，Go runtime 随 C++ 进程一起启动。

---

## 二、与 Hermind 当前方案的对比

| 维度 | Hermind 当前（HTTP） | 参考项目（CGO+Protobuf） |
|------|---------------------|------------------------|
| **进程模型** | 双进程：Qt 前端 + Go 后端 | 单进程：Go 作为静态库链接到 Qt |
| **通信方式** | HTTP/REST over localhost | 直接函数调用（in-process） |
| **数据格式** | JSON | Protobuf binary |
| **序列化开销** | JSON encode/decode | Protobuf marshal/unmarshal |
| **网络开销** | TCP loopback | 零（内存拷贝） |
| **启动开销** | 需要启动 Go 子进程，等待端口 | Go runtime 随 Qt 进程一起启动 |
| **延迟** | ~1-5ms（localhost） | ~0.01-0.1ms（函数调用） |
| **Go 代码改造量** | 无需改造（标准 HTTP） | **巨大**（需改为 `package main` + `//export`） |
| **C++ 代码量** | 中等（HTTP client） | **大**（需写 Protobuf 封装层 + CGO 桥接） |
| **调试难度** | 简单（可独立运行 Go backend） | **困难**（Go 在 C++ 进程中，CGO 调试复杂） |
| **跨平台编译** | 简单（分别编译两个 exe） | **复杂**（需为每个平台编译 Go 静态库） |
| **依赖管理** | 简单 | 需引入 protobuf C++ 库 |
| **崩溃隔离** | ✅ 好（Go 崩溃不影响 Qt） | ❌ 差（Go panic 会拖垮整个进程） |
| **内存泄漏追踪** | 简单（独立进程） | **困难**（C.malloc / C.free 需手动管理） |
| **热重载** | ✅ 可独立重启 Go backend | ❌ 不可（需重启整个应用） |

---

## 三、Hermind 改造的工程量评估

### 3.1 Go 侧需改造的内容

Hermind 的 Go 后端是一个完整的 Web 服务器（`go-chi` 路由 + 大量 HTTP handler），要改为 CGO 静态库，需要：

1. **入口点改造**：
   - 当前：`cmd/hermind/main.go` 启动 HTTP server
   - 改造后：改为 `package main` + `func main() {}`，所有 handler 逻辑改为 `//export` 函数

2. **所有 API 接口重写**：
   - 当前有约 **15+ 个 HTTP handler**（conversation, config, providers, health 等）
   - 每个 handler 需要定义对应的 Protobuf message（请求 + 响应）
   - 每个 handler 需要写一个 `//export` 的 CGO 包装函数

3. **依赖注入问题**：
   - 当前 Go 后端使用复杂的依赖注入（App 对象、storage、provider、agent 等）
   - CGO 静态库没有"启动"概念，需要在第一个 `//export` 调用前完成所有初始化
   - 需要重构为懒加载或显式初始化模式

4. **并发和上下文**：
   - HTTP handler 有 `http.Request.Context()` 用于取消/超时
   - CGO 函数调用没有内置 context，需要显式传递 timeout 参数

5. **第三方库兼容性**：
   - 部分库可能不支持 `buildmode=c-archive`（尤其是使用 `os.Exit` 或 `runtime` 特殊操作的库）

### 3.2 C++ 侧需新增的内容

1. **Protobuf 定义文件**：为所有 Go API 定义 `.proto` 文件（请求/响应消息）
2. **C++ Protobuf 编译**：CMake 集成 `protoc` 生成 C++ 代码
3. **CGO 桥接层**：类似参考项目的 `go_interface.h/cpp`
   - `Call_Go_Func` 通用调用封装
   - `CGoInterface` 回调分发管理
   - 内存管理（`C.malloc` → `free`）
4. **QML 集成**：将同步/异步调用封装为 QML 可调用的 C++ 对象
5. **CMake 构建链**：
   - 自定义命令：构建前执行 `go build -buildmode=c-archive`
   - 链接生成的 `.a` 库
   - Windows 下 MinGW 与 CGO 的兼容性问题

### 3.3 预估工作量

| 任务 | 预估时间 |
|------|---------|
| Protobuf 协议设计（所有 API） | 2-3 天 |
| Go 侧 CGO 改造（核心 API） | 5-7 天 |
| C++ 侧桥接层实现 | 3-4 天 |
| CMake 构建链改造（含跨平台） | 2-3 天 |
| 调试与问题修复 | 3-5 天 |
| **总计** | **15-22 天** |

---

## 四、三种可行方案对比

### 方案 A：全面改造为 CGO 静态库（参考项目模式）

**优点**：
- 零网络开销，最低延迟
- 单进程，部署简单（一个 exe）
- 数据交换用 Protobuf，比 JSON 更高效

**缺点**：
- 工程量巨大（15-22 天）
- 调试极其困难（CGO + 单进程）
- 崩溃隔离差（Go panic → 整个应用崩溃）
- 需要引入 C++ Protobuf 库（增加二进制体积）
- 失去 HTTP 生态（中间件、测试工具、curl 调试等）

**适用场景**：对延迟极度敏感（<1ms）、API 数量少且稳定的项目。

### 方案 B：保留 HTTP，但改为内嵌启动（Unix Domain Socket / Named Pipe）

**改进点**：
- Qt 启动时将 Go backend 编译为可执行文件，随 Qt 一起打包
- 使用 **Unix Domain Socket**（Linux/macOS）或 **Named Pipe**（Windows）替代 TCP loopback
- 通信协议保持 HTTP/JSON 不变

**优点**：
- 延迟降低（UDS 比 TCP loopback 快 2-3 倍）
- Go 代码**无需任何改造**
- 保留 HTTP 生态和调试便利性
- 崩溃隔离好

**缺点**：
- 仍然是双进程
- 需要处理 Go 进程的启动/监控/重启

**预估工作量**：2-3 天（主要是 Qt 侧启动逻辑 + UDS/Named Pipe 配置）

### 方案 C：局部 CGO（只将高频/低延迟操作改为 CGO）

**思路**：
- 保留现有 HTTP 通信作为主力
- 仅将**真正需要低延迟**的操作（如实时输入检测、高频状态查询）改为 CGO 静态库
- 大部分业务仍走 HTTP

**优点**：
- 渐进式改造，风险可控
- 核心路径延迟降低
- 不影响现有 HTTP API

**缺点**：
- 架构混合，维护两套通信机制
- 边界设计需要仔细考虑

**预估工作量**：5-8 天（取决于需要 CGO 化的 API 数量）

---

## 五、建议

### 我的推荐：方案 B（UDS/Named Pipe + 内嵌 Go binary）

**理由**：

1. **ROI 最高**：2-3 天工作量即可获得显著收益（延迟降低、部署简化），而方案 A 需要 15-22 天且收益边际递减。

2. **Go 代码零改造**：Hermind 的 Go 后端是一个功能完整的 Web 服务器，有大量 HTTP handler 和中间件，改造为 CGO 的代价与收益不成正比。

3. **调试友好**：保留独立 Go 进程，开发时仍可独立启动 backend 进行调试，这是日常开发中极其重要的便利性。

4. **崩溃隔离**：桌面应用中，Go 逻辑（尤其是 AI 推理相关）如果发生 panic，不应该拖垮整个 Qt UI。

5. **Protobuf 不是银弹**：对于桌面应用内的通信，HTTP/JSON 的 overhead（~1-5ms）相对于 AI 推理的延迟（秒级）完全可以忽略。

### 如果坚持方案 A（全面 CGO 改造）

需要回答以下问题：
- Hermind 的 API 是否已经冻结？（Protobuf 协议一旦确定，变更成本极高）
- 是否有 2-3 周的专注开发时间？
- 团队是否熟悉 CGO 调试？（这非常痛苦）
- 是否有跨平台编译的 CI 支持？（Windows MinGW + CGO  notoriously tricky）

---

## 六、结论

参考项目 `models_gen` 的通信方式本质上是 **CGO 静态库 + Protobuf 序列化**，不是通过网络 gRPC。这种方案适合 API 数量少、对延迟极度敏感的场景。

对于 Hermind 这样有 **15+ API endpoint**、功能持续迭代、AI 推理为主的桌面应用，**保留 HTTP 协议并优化传输层（UDS/Named Pipe）**是更务实的选择。如果需要进一步降低延迟，可以考虑**方案 C 的局部 CGO**作为补充。
