# go-desktop-interface 设计文档

## 目标

新建 `cmd/go-desktop-interface`，将 Hermind 的 Go 后端编译为 C 静态库，通过 CGO 供 Qt/C++ 直接调用。保留现有的 `cmd/hermind` HTTP 版本不变。

---

## 架构总览

```
┌──────────────────────────────────────────────────────────────────────┐
│                    hermind-desktop.exe (单一进程)                      │
│                                                                      │
│  ┌─────────────────┐        CGO         ┌────────────────────────┐  │
│  │  Qt/C++ 前端     │  ◄──────────────► │  Go 静态库 (.a)         │  │
│  │                 │   JSON bytes       │  cmd/go-desktop-interface│ │
│  │ HermindCGOClient│                    │                        │  │
│  │ • get()         │                    │ //export HermindInit() │  │
│  │ • post()        │                    │ //export HermindCall() │  │
│  │ • getStream()   │ ◄── C callback ── │ //export HermindFree() │  │
│  └─────────────────┘   (SSE chunks)     └────────────────────────┘  │
│         │                                    │                        │
│         │                                    │ 复用 cli.App + api.Server│
│         │                                    │ 的业务逻辑              │
│         │                            ┌───────┴──────────┐             │
│         │                            │ config, agent,    │             │
│         │                            │ storage, provider │             │
│         │                            │ (不变)            │             │
│         │                            └───────────────────┘             │
│         │                                                              │
│  ┌──────▼──────────┐                                                  │
│  │  QML / JS 前端   │  ← QML 代码不变，只是 hermindClient 的          │
│  │  (完全不变)      │    实现从 HTTP 换成了 CGO                        │
│  └─────────────────┘                                                  │
└──────────────────────────────────────────────────────────────────────┘

HTTP 版本（保留不变）：
  cmd/hermind/main.go → hermind.exe desktop → HTTP server
```

---

## 设计原则

1. **Go 侧零侵入**：不修改 `api/`、`cli/`、`agent/` 等任何现有包的代码，所有 CGO 封装在新建的 `cmd/go-desktop-interface/` 中完成
2. **C++ 侧 API 兼容**：`HermindCGOClient` 保持与 `HermindClient` 完全相同的公开接口，QML 代码无需任何改动
3. **JSON 序列化**：复用现有 JSON DTO，不引入 Protobuf
4. **统一调用入口**：单个 `HermindCall` 函数通过 `method` + `path` 路由分发，而非每个 API 单独导出

---

## Go 侧设计

### 文件结构

```
cmd/go-desktop-interface/
├── main.go          # CGO 导出函数、初始化、统一调用入口
├── router.go        # method + path → handler 路由表
├── stream.go        # SSE 流式回调管理
└── build.go         # build constraints（确保 buildmode=c-archive）
```

### CGO 导出函数

```go
package main

/*
#include <stdlib.h>
*/
import "C"

//export HermindInit
func HermindInit(configPathC *C.char) *C.char
// 初始化 Hermind App 和 Server。configPathC 为配置文件路径，空字符串使用默认路径。
// 返回 JSON 状态字符串（C.malloc 分配），调用方需用 HermindFree 释放。

//export HermindCall
func HermindCall(methodC *C.char, pathC *C.char, bodyC *C.char, bodyLen C.int, respLen *C.int) unsafe.Pointer
// 统一 API 调用入口。
// methodC: "GET" / "POST" / "PUT" / "DELETE"
// pathC:    API path，如 "/api/config"
// bodyC:    请求体 JSON bytes（可为 nil）
// bodyLen:  请求体长度
// respLen:  输出参数，响应 JSON bytes 的长度
// 返回:     响应 JSON bytes 的指针（C.malloc 分配），调用方需用 HermindFree 释放。

//export HermindFree
func HermindFree(p unsafe.Pointer)
// 释放 HermindInit 或 HermindCall 返回的内存。

//export HermindSetStreamCallback
func HermindSetStreamCallback(callback unsafe.Pointer)
// 注册 C++ 侧的流式回调函数指针，供 SSE 推送使用。

func main() {}
```

### 内部路由实现

`HermindCall` 内部维护一个路由表，将 `method + path` 映射到对应的 `api.Server` handler：

```go
var routes = []route{
    {method: "GET",  path: "/api/status",               handler: handleStatus},
    {method: "GET",  path: "/api/config",               handler: handleConfigGet},
    {method: "PUT",  path: "/api/config",               handler: handleConfigPut},
    // ... 所有 API endpoint
}
```

handler 函数签名：
```go
type handlerFunc func(s *api.Server, body map[string]any) (any, error)
```

handler 内部直接调用 `api.Server` 的私有方法（或提取公共方法）。如果 `api.Server` 的 handler 是私有的，可以在 `cmd/go-desktop-interface` 中通过内联调用或提取公共方法来实现。

### SSE 流式处理

SSE 不走 `HermindCall`，而是通过独立的流式机制：

1. C++ 侧调用 `HermindSetStreamCallback`，传入 C 函数指针
2. Go 侧保存该指针
3. 当流式事件发生时，Go 通过 `Cgo` 调用该 C 函数指针，将事件数据推送给 C++
4. C++ 侧收到数据后，通过 Qt 信号槽分发给 `AppState`

```go
// Go 侧推送 SSE chunk
func pushStreamChunk(eventType string, data string) {
    if streamCallback == nil { return }
    eventJSON, _ := json.Marshal(map[string]string{
        "type": eventType,  // "chunk", "done", "error"
        "data": data,
    })
    // 通过保存的 C 函数指针回调 C++
}
```

### 复用初始化逻辑

`HermindInit` 复用 `cli/desktop.go` 和 `cli/app.go` 的初始化逻辑：

```go
func HermindInit(configPathC *C.char) *C.char {
    configPath := C.GoString(configPathC)
    
    // 1. 创建 cli.App（复用 cli.NewApp）
    app = cli.NewApp(cli.WithConfigPath(configPath))
    
    // 2. 构建 EngineDeps（复用 cli.BuildEngineDeps）
    deps, err := cli.BuildEngineDeps(app)
    if err != nil { ... }
    
    // 3. 创建 api.Server
    server, err = api.NewServer(&api.ServerOpts{
        Config:     app.Config(),
        ConfigPath: configPath,
        Storage:    app.Storage(),
        Deps:       *deps,
        // ...
    })
    if err != nil { ... }
    
    // 4. 启动 gateway pump（如果需要）
    // server.StartGateway(ctx)
    
    // 返回初始化状态
    status := map[string]string{"status": "ok"}
    data, _ := json.Marshal(status)
    return (*C.char)(C.malloc(C.size_t(len(data)))) // copy data
}
```

---

## C++ 侧设计

### 文件结构

```
desktop/src/
├── HermindCGOClient.h    # 替换 HermindClient（API 完全兼容）
├── HermindCGOClient.cpp  # CGO 调用实现
├── HermindProcess.h      # 保留但可选（HTTP 版本用）
├── HermindProcess.cpp    # 保留但可选
└── main.cpp              # 改造初始化流程
```

### HermindCGOClient 类

```cpp
class HermindCGOClient : public QObject {
    Q_OBJECT
public:
    explicit HermindCGOClient(QObject *parent = nullptr);
    
    // C++ API（与 HermindClient 完全一致）
    void get(const QString &path, Callback callback);
    void post(const QString &path, const QJsonObject &body, Callback callback);
    void put(const QString &path, const QJsonObject &body, Callback callback);
    void delete_(const QString &path, Callback callback);
    void upload(const QString &path, const QByteArray &data,
                const QString &fileName, const QString &mimeType,
                Callback callback);
    QNetworkReply* getStream(const QString &path);
    
    // QML API（与 HermindClient 完全一致）
    Q_INVOKABLE void get(const QString &path, QJSValue callback);
    Q_INVOKABLE void post(const QString &path, QJsonObject body, QJSValue callback);
    Q_INVOKABLE void put(const QString &path, QJsonObject body, QJSValue callback);
    Q_INVOKABLE void delete_(const QString &path, QJSValue callback);
    Q_INVOKABLE void upload(const QString &path, const QByteArray &data,
                            const QString &fileName, const QString &mimeType,
                            QJSValue callback);
    Q_INVOKABLE QNetworkReply* getStream(const QString &path);
    
    QString baseUrl() const { return QStringLiteral("cgo://internal"); }

signals:
    // 新增：SSE 流式事件信号
    void streamEvent(const QString &eventType, const QString &data);

private:
    static QJsonObject doCall(const QString &method, const QString &path,
                               const QJsonObject &body = QJsonObject());
};
```

### 同步调用实现

```cpp
QJsonObject HermindCGOClient::doCall(const QString &method, const QString &path,
                                      const QJsonObject &body) {
    QByteArray bodyBytes;
    const char* bodyPtr = nullptr;
    int bodyLen = 0;
    if (!body.isEmpty()) {
        bodyBytes = QJsonDocument(body).toJson(QJsonDocument::Compact);
        bodyPtr = bodyBytes.constData();
        bodyLen = bodyBytes.length();
    }
    
    int respLen = 0;
    void* resp = HermindCall(
        method.toUtf8().constData(),
        path.toUtf8().constData(),
        bodyPtr,
        bodyLen,
        &respLen
    );
    
    QByteArray respData((const char*)resp, respLen);
    HermindFree(resp);
    
    QJsonDocument doc = QJsonDocument::fromJson(respData);
    return doc.object();
}
```

### SSE 流式模拟

由于 `getStream()` 需要返回 `QNetworkReply*`，但实际不走 HTTP，需要创建一个模拟的 `QNetworkReply` 子类：

```cpp
class CGOStreamReply : public QNetworkReply {
    Q_OBJECT
public:
    CGOStreamReply(const QString &path, QObject *parent = nullptr);
    void appendChunk(const QByteArray &data);
    void finish();
    void setError(const QString &error);
    
    qint64 readData(char *data, qint64 maxlen) override;
    void abort() override;
};
```

`HermindCGOClient::getStream()` 创建 `CGOStreamReply` 实例并返回，Go 侧通过回调将数据写入该实例的 buffer，触发 `readyRead` 信号。

### main.cpp 初始化改造

```cpp
// 改造前（HTTP 版本）：
HermindProcess backend;
HermindClient *client = nullptr;
// ... 等待 backendReady(port) 信号后创建 HermindClient

// 改造后（CGO 版本）：
// 1. 调用 Go 初始化
QByteArray configPath = /* 获取配置文件路径 */;
char* status = HermindInit(configPath.constData());
HermindFree(status);

// 2. 直接创建 CGO Client
HermindCGOClient *client = new HermindCGOClient(&engine);
appState.setClient(client);
engine.rootContext()->setContextProperty("hermindClient", client);

// 3. 注册 SSE 回调
HermindSetStreamCallback((void*)C_StreamCallback);

// 4. 启动 AppState
appState.boot();
```

### C 回调函数

```cpp
// go_callbacks.cpp
extern "C" {
    // 供 Go 侧调用的 C 回调
    void C_StreamCallback(const char* eventType, const char* data) {
        QString typeStr = QString::fromUtf8(eventType);
        QString dataStr = QString::fromUtf8(data);
        // 通过 QMetaObject::invokeMethod 安全地跨线程 emit 信号
        QMetaObject::invokeMethod(HermindCGOClient::instance(), "streamEvent",
            Qt::QueuedConnection,
            Q_ARG(QString, typeStr), Q_ARG(QString, dataStr));
    }
}
```

---

## CMake 构建链改造

### 新增：Go 静态库编译

```cmake
# 定义 Go 源文件列表（用于依赖追踪）
file(GLOB_RECURSE GO_SOURCES ${CMAKE_SOURCE_DIR}/../cmd/go-desktop-interface/*.go)

# 自定义命令：编译 Go 静态库
add_custom_command(
    OUTPUT ${CMAKE_BINARY_DIR}/libgo-desktop-interface.a
           ${CMAKE_BINARY_DIR}/go-desktop-interface.h
    COMMAND ${CMAKE_COMMAND} -E env 
        CGO_ENABLED=1
        GOOS=windows
        GOARCH=amd64
        CC=${MINGW_CC}
        go build -buildmode=c-archive
            -o ${CMAKE_BINARY_DIR}/libgo-desktop-interface.a
            ${CMAKE_SOURCE_DIR}/../cmd/go-desktop-interface
    DEPENDS ${GO_SOURCES}
    WORKING_DIRECTORY ${CMAKE_SOURCE_DIR}/..
    COMMENT "Building Go desktop interface static library..."
)

add_custom_target(go-desktop-interface DEPENDS
    ${CMAKE_BINARY_DIR}/libgo-desktop-interface.a
)
```

### 链接配置

```cmake
# 将生成的头文件目录加入 include 路径
target_include_directories(hermind-desktop PRIVATE ${CMAKE_BINARY_DIR})

# 链接 Go 静态库
target_link_libraries(hermind-desktop PRIVATE
    ${CMAKE_BINARY_DIR}/libgo-desktop-interface.a
    # Windows 下 CGO 需要的系统库
    ws2_32
    winmm
    # MinGW 的 pthread（CGO 需要）
    pthread
)

# 确保 C++ 目标在 Go 静态库之后构建
add_dependencies(hermind-desktop go-desktop-interface)
```

---

## 内存管理约定

| 分配者 | 释放者 | 说明 |
|---|---|---|
| Go (`C.malloc`) | C++ (`HermindFree`) | HermindInit / HermindCall 的返回值 |
| C++ (`qstrdup` / `toUtf8`) | Go（自动 GC） | 传入 HermindCall 的字符串参数 |
| Go (`C.CString`) | Go (`C.free`) | SSE 回调中 Go 创建的临时字符串 |

**规则**：
1. Go 返回的所有数据必须用 `C.malloc` 分配，C++ 侧使用完后必须调用 `HermindFree`
2. C++ 传入 Go 的字符串/bytes 在 Go 函数返回后即可被 GC（Go 侧会在函数内复制）
3. SSE 回调中，Go 用 `C.CString` 创建临时字符串，回调后立即 `C.free`

---

## 错误处理

### Go 侧 → C++ 侧错误传递

`HermindCall` 的返回值统一为 JSON：

```json
// 成功响应
{"ok": true, "data": {...}}

// 错误响应
{"ok": false, "error": "error message", "code": 500}
```

C++ 侧检查 `"ok"` 字段，如果为 `false`，将 `"error"` 作为错误信息传递给 callback。

### Go 内部错误

- 路由未找到：返回 `{"ok": false, "error": "not found", "code": 404}`
- handler panic：recover 后返回 `{"ok": false, "error": "internal error", "code": 500}`
- 初始化失败：`HermindInit` 返回 `{"status": "error", "error": "..."}`

---

## 线程模型

```
┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│   Qt 主线程   │     │   Go 线程    │     │  Go goroutine │
│  (UI 事件)   │     │ (CGO 调用)   │     │ (SSE 推送)   │
└──────┬───────┘     └──────┬───────┘     └──────┬───────┘
       │                    │                    │
       │ HermindCall()      │                    │
       │ ─────────────────► │                    │
       │                    │ handler()          │
       │                    │ ─────────────────► │
       │                    │                    │ stream chunk
       │                    │ ◄───────────────── │
       │ 返回结果            │                    │
       │ ◄───────────────── │                    │
       │                    │                    │ Cgo callback
       │                    │                    │ ─────────────────►
       │                    │                    │   Qt 信号槽队列
       │  callback(result)  │                    │
       │  (QueuedConnection) │                   │
```

- `HermindCall` 在 Qt 主线程调用，Go 侧在独立的 CGO 线程执行，完成后返回
- SSE 推送从 Go goroutine 通过 Cgo 回调到 C++，再通过 `Qt::QueuedConnection` 切换到 Qt 主线程
- 所有 UI 更新都在 Qt 主线程完成

---

## 风险与缓解

| 风险 | 影响 | 缓解措施 |
|---|---|---|
| MinGW + CGO 编译失败 | 高 | 提前验证 `go build -buildmode=c-archive` 在 MinGW 环境下可用 |
| Go runtime 初始化延迟 | 中 | `HermindInit` 在 splash/loading 阶段调用，UI 显示初始化进度 |
| Go panic 拖垮整个应用 | 高 | 所有 handler 用 `defer recover()` 包装，panic 转为错误响应 |
| 内存泄漏（C.malloc 未 free） | 中 | C++ 侧用 RAII 包装（`QScopedPointer` + 自定义 deleter） |
| SSE 回调跨线程问题 | 中 | 使用 `Qt::QueuedConnection`，避免直接操作 UI |
| 二进制体积增大 | 低 | Go 静态库约 10-30MB，可接受 |

---

## 实现阶段

### Phase 1：Go 侧最小可用集（MVP）
- 创建 `cmd/go-desktop-interface/`
- 实现 `HermindInit` + `HermindCall` + `HermindFree`
- 仅暴露 `GET /api/status` 和 `GET /health` 两个 API
- 验证 CGO 编译和 C++ 调用链路

### Phase 2：核心 API 覆盖
- 添加 `GET /api/config`、`PUT /api/config`、`GET /api/conversation`、`POST /api/conversation/messages`
- 实现 SSE 流式回调
- C++ 侧完成 `HermindCGOClient` 全部 API

### Phase 3：完整 API 覆盖
- 添加剩余所有 API endpoint
- 文件上传支持
- 全面测试

### Phase 4：构建链完善
- CMake 集成 `go build -buildmode=c-archive`
- Windows / macOS / Linux 跨平台配置
- 发布打包脚本

---

## 附录：完整 API 映射表

| HTTP Method | Path | CGO 调用示例 |
|---|---|---|
| `GET` | `/health` | `HermindCall("GET", "/health", nil, 0, &len)` |
| `GET` | `/api/status` | `HermindCall("GET", "/api/status", nil, 0, &len)` |
| `GET` | `/api/model/info` | `HermindCall("GET", "/api/model/info", nil, 0, &len)` |
| `GET` | `/api/config` | `HermindCall("GET", "/api/config", nil, 0, &len)` |
| `PUT` | `/api/config` | `HermindCall("PUT", "/api/config", body, len, &len)` |
| `GET` | `/api/config/schema` | `HermindCall("GET", "/api/config/schema", nil, 0, &len)` |
| `GET` | `/api/conversation` | `HermindCall("GET", "/api/conversation", nil, 0, &len)` |
| `POST` | `/api/conversation/messages` | `HermindCall("POST", "/api/conversation/messages", body, len, &len)` |
| `POST` | `/api/conversation/cancel` | `HermindCall("POST", "/api/conversation/cancel", nil, 0, &len)` |
| `PUT` | `/api/conversation/messages/{id}` | `HermindCall("PUT", path, body, len, &len)` |
| `DELETE` | `/api/conversation/messages/{id}` | `HermindCall("DELETE", path, nil, 0, &len)` |
| `POST` | `/api/conversation/messages/{id}/regenerate` | `HermindCall("POST", path, nil, 0, &len)` |
| `GET` | `/api/sse` | `getStream("/api/sse")` → CGO callback |
| `GET` | `/api/tools` | `HermindCall("GET", "/api/tools", nil, 0, &len)` |
| `GET` | `/api/skills` | `HermindCall("GET", "/api/skills", nil, 0, &len)` |
| `GET` | `/api/skills/stats` | `HermindCall("GET", "/api/skills/stats", nil, 0, &len)` |
| `GET` | `/api/providers` | `HermindCall("GET", "/api/providers", nil, 0, &len)` |
| `POST` | `/api/providers/{name}/models` | `HermindCall("POST", path, body, len, &len)` |
| `POST` | `/api/providers/{name}/test` | `HermindCall("POST", path, body, len, &len)` |
| `POST` | `/api/fallback_providers/{index}/models` | `HermindCall("POST", path, body, len, &len)` |
| `POST` | `/api/auxiliary/models` | `HermindCall("POST", path, body, len, &len)` |
| `POST` | `/api/auxiliary/test` | `HermindCall("POST", path, body, len, &len)` |
| `GET` | `/api/memory/stats` | `HermindCall("GET", "/api/memory/stats", nil, 0, &len)` |
| `GET` | `/api/memory/health` | `HermindCall("GET", "/api/memory/health", nil, 0, &len)` |
| `GET` | `/api/memory/report` | `HermindCall("GET", "/api/memory/report", nil, 0, &len)` |
| `GET` | `/api/memory/{id}` | `HermindCall("GET", path, nil, 0, &len)` |
| `POST` | `/api/upload` | `HermindCall("POST", "/api/upload", body, len, &len)` |
| `POST` | `/api/feedback` | `HermindCall("POST", "/api/feedback", body, len, &len)` |
| `GET` | `/api/suggestions` | `HermindCall("GET", "/api/suggestions", nil, 0, &len)` |
| `POST` | `/api/tts` | `HermindCall("POST", "/api/tts", body, len, &len)` |
| `POST` | `/api/render` | `HermindCall("POST", "/api/render", body, len, &len)` |
| `POST` | `/api/render/math` | `HermindCall("POST", "/api/render/math", body, len, &len)` |
| `POST` | `/api/render/mermaid` | `HermindCall("POST", "/api/render/mermaid", body, len, &len)` |
