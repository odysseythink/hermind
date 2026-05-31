# Native Embedding (ONNX) 设计文档

> 生成时间: 2026-05-28
> 状态: 已审批
> 技术路线: Cybertron 纯 Go (`github.com/nlpodyssey/cybertron`)
> 对标: anything-llm v1.13.0 `server/utils/EmbeddingEngines/native/`

---

## 1. 背景与目标

Hermind 后端（Go）目前所有 embedding provider 都依赖外部 API（OpenAI、Cohere、Voyage 等），没有本地离线 embedding 能力。anything-llm 的 **Native Embedding** 是核心差异化功能：在纯 CPU 环境下本地运行 embedding，无需外部 API，隐私性强。

本设计目标是在 Hermind 后端实现与 anything-llm 等价的 Native Embedding 功能，前端零改动。

---

## 2. 技术路线决策

| 方案 | 结论 |
|------|------|
| **A. Cybertron 纯 Go** | ✅ **选定**。`github.com/nlpodyssey/cybertron` v0.2.1 已在 go.mod 中，纯 Go 无 CGO，零外部依赖，可从 HuggingFace 加载 BERT 模型做 Text Embedding |
| B. ONNX Runtime Go | ❌ 需要 CGO + ONNX Runtime C 库，跨平台编译复杂 |
| C. 外部 Python 进程 | ❌ 引入 Python 依赖，违背单二进制部署哲学 |
| D. 复用 Ollama | ❌ 仍需单独运行外部服务，不是真正的 Native |

---

## 3. 架构概述

### 3.1 新增文件

| 文件 | 说明 |
|------|------|
| `backend/internal/embedder/native.go` | `NativeEmbedder` 实现 `Embedder` interface |
| `backend/internal/embedder/native_models.go` | 3 个预配置模型的元数据、HF repo、维度、prefix、并发限制 |

### 3.2 修改文件

| 文件 | 修改 |
|------|------|
| `backend/internal/embedder/factory.go` | `switch` 新增 `case "native"`，构造 `NativeEmbedder` |
| `backend/internal/handlers/system.go` | `CustomModels` handler 新增 `case "native-embedder"` 返回模型列表 |
| `backend/internal/config/config.go` | 新增 `NativeEmbeddingModel` 配置项 |
| `backend/cmd/server/main.go` | 确保 native embedder 初始化路径正确 |

### 3.3 零改动文件

- **前端**: `NativeEmbeddingOptions/index.jsx`、`EmbeddingPreference/index.jsx` 已完整保留 anything-llm 的 UI，无需任何修改
- **向量数据库层**: 所有 vectordb provider 通过 `Embedder` interface 调用，对 native embedder 无感知

---

## 4. 模型配置

### 4.1 数据模型

```go
// backend/internal/embedder/native_models.go
type NativeModelInfo struct {
    ID                      string
    Name                    string
    Description             string
    Lang                    string
    Size                    string
    ModelCard               string
    HFRepo                  string        // HuggingFace repo for cybertron
    Dimensions              int
    MaxConcurrentChunks     int           // 并发批次限制，防止 OOM
    EmbeddingMaxChunkLength int           // 字符数限制（对齐 anything-llm 的字符策略）
    ChunkPrefix             string        // 文档 embedding 前缀（如 "search_document: "）
    QueryPrefix             string        // 查询 embedding 前缀（如 "search_query: "）
}
```

### 4.2 预配置模型列表

| ID | 对齐 anything-llm | HF Repo | 维度 | 大小 | Chunk Prefix | Query Prefix |
|----|-------------------|---------|------|------|--------------|--------------|
| `sentence-transformers/all-MiniLM-L6-v2` | `Xenova/all-MiniLM-L6-v2` | `sentence-transformers/all-MiniLM-L6-v2` | 384 | ~23MB | `""` | `""` |
| `sentence-transformers/all-mpnet-base-v2` | `Xenova/nomic-embed-text-v1` | `sentence-transformers/all-mpnet-base-v2` | 768 | ~139MB | `""` | `""` |
| `sentence-transformers/distiluse-base-multilingual-cased-v1` | `MintplexLabs/multilingual-e5-small` | `sentence-transformers/distiluse-base-multilingual-cased-v1` | 512 | ~135MB | `""` | `""` |

> **模型选择说明**: anything-llm 使用 ONNX 格式的 sentence-transformers 模型（MiniLM、nomic、e5）。Cybertron 原生支持 BERT 架构，sentence-transformers 的模型（MiniLM、MPNet、DistilUSE）在架构层面与 BERT 兼容，cybertron 的 `ConvertHuggingFacePreTrained` 应能加载。如果在实现阶段发现某个模型不兼容，将替换为等价的 BERT-base sentence embedding 模型。

### 4.3 Config 新增字段

```go
// backend/internal/config/config.go
NativeEmbeddingModel string `env:"NATIVE_EMBEDDING_MODEL" envDefault:"sentence-transformers/all-MiniLM-L6-v2"`
```

默认模型与 anything-llm 一致（`all-MiniLM-L6-v2`）。

---

## 5. 核心组件与数据流

### 5.1 NativeEmbedder 结构

```go
type NativeEmbedder struct {
    modelInfo NativeModelInfo
    cacheDir  string            // <StorageDir>/models
    modelPath string            // cacheDir + model ID 子目录
    model     *bert.Model       // cybertron BERT 模型实例
    tokenizer *tokenizer.BPE    // cybertron tokenizer
    dims      int
}
```

### 5.2 EmbedTexts 流程

```
输入 texts []string
  │
  ├─→ 应用 chunk prefix（如 "search_document: "）到每个 text
  │
  ├─→ 按 EmbeddingMaxChunkLength 截断超长文本
  │
  ├─→ 分批次：每批 MaxConcurrentChunks 个文本
  │     │
  │     └─→ 对每批调用 cybertron Vectorize(text, ReduceMean)
  │           │
  │           └─→ 归一化向量（L2 norm = 1）
  │
  ├─→ 大文档（>100 个 chunks）时，中间结果写入 tempfile
  │     防止 Go GC 前内存峰值过高
  │
  └─→ 展平所有批次结果 → [][]float32
```

### 5.3 EmbedQuery 流程

```
输入 query string
  │
  ├─→ 应用 query prefix（如 "search_query: "）
  │
  └─→ 调用 EmbedTexts([]string{query}) → 取第一个
```

### 5.4 模型加载（懒加载 + 缓存）

```go
func (e *NativeEmbedder) loadModel() error {
    if e.model != nil { return nil }  // 已加载，直接返回
    if !modelExists(e.modelPath) {
        // 首次使用：从 HuggingFace 下载并转换为 spaGO 格式
        log("Downloading %s from HuggingFace...", e.modelInfo.HFRepo)
        err := bert.ConvertHuggingFacePreTrained(e.modelPath)
        // ...
    }
    model, err := bert.LoadModel(e.modelPath)
    tokenizer, err := tokenizer.Load(e.modelPath)
    e.model = model
    e.tokenizer = tokenizer
    return nil
}
```

模型下载到 `<StorageDir>/models/{modelID}/`，首次下载后复用缓存。

### 5.5 与 anything-llm 行为对齐

| anything-llm 行为 | Hermind 实现 |
|-------------------|--------------|
| `pipeline("feature-extraction", {pooling: "mean", normalize: true})` | `Vectorize(text, ReduceMean)` + L2 归一化 |
| `maxConcurrentChunks` 限制 | 同样实现，防止 OOM |
| 临时文件存储中间结果 | 大文档时启用 |
| 模型下载到 `storage/models/` | 缓存到 `<StorageDir>/models/` |
| 首次下载慢，后续复用 | 懒加载 + 文件缓存 |

---

## 6. API 接口

### 6.1 获取可用模型列表

**前端调用**: `System.customModels("native-embedder")`

**已有端点**: `POST /api/system/custom-models`

**新增逻辑**（`system.go` `CustomModels` handler）:

```go
case "native-embedder":
    models := native.AvailableModels() // 返回 []gin.H{id,name,description,lang,size,modelCard}
    c.JSON(http.StatusOK, gin.H{"models": models, "error": nil})
```

返回格式对齐 anything-llm 前端期望：

```json
{
  "models": [
    {
      "id": "sentence-transformers/all-MiniLM-L6-v2",
      "name": "all-MiniLM-L6-v2",
      "description": "A lightweight and fast model for embedding text...",
      "lang": "English",
      "size": "23MB",
      "modelCard": "https://huggingface.co/sentence-transformers/all-MiniLM-L6-v2"
    }
  ],
  "error": null
}
```

### 6.2 Embedding 调用

无需新增端点。Native embedder 通过现有 `Embedder` interface 被向量化流程（`EmbedWorkerJob`、`VectorSearchService` 等）透明调用。

---

## 7. 错误处理

| 场景 | 处理策略 |
|------|----------|
| **模型下载失败**（HF 不可达/网络问题） | 返回 `fmt.Errorf("native embedder: failed to download model %s: %w", modelID, err)`，错误传播到 embedder 初始化，前端显示 "模型下载失败" |
| **模型格式不兼容**（cybertron 无法转换） | 初始化时检测，返回明确错误。预配置模型列表已规避此风险 |
| **单批文本过长**（超过 EmbeddingMaxChunkLength） | 在 batch 前按字符数截断，记录 `mlog.Warn` |
| **内存溢出风险**（超大文档） | `maxConcurrentChunks` 限制 + 临时文件中间存储 |
| **并发调用** | `NativeEmbedder` 实例线程安全：模型加载用 `sync.Once`，推理阶段只读共享模型 |

---

## 8. 测试策略

| 测试类型 | 文件 | 内容 |
|----------|------|------|
| 单元测试 | `native_models_test.go` | 验证模型配置完整性（每个模型都有 HFRepo、Dimensions > 0） |
| 单元测试 | `native_test.go` | 测试 prefix 应用、文本截断、批次分割逻辑（mock 模型） |
| 集成测试 | `native_integration_test.go` | 实际下载模型 + 推理（标记 `-tags=integration`，CI 跳过） |
| Factory 测试 | `factory_test.go` | 验证 `EmbeddingEngine="native"` 能正确构造 `NativeEmbedder` |

---

## 9. 性能预期

基于 anything-llm 的基准（t3.small, 2GB RAM, 1vCPU）：
- all-MiniLM-L6-v2：约 30% 内存占用，>100K 词文档可完成
- Go 的 GC 策略比 Node.js V8 更可控，理论上 OOM 风险更低

---

## 10. 实现后前端验证清单

- [ ] 设置页面 → Embedding Preference → 选择 "Hermind Embedder" → 模型下拉框显示 3 个模型
- [ ] 选择模型后保存设置成功
- [ ] 上传文档后 embedding worker 使用 native embedder 生成向量
- [ ] 聊天时 RAG 检索使用 native embedder 的 query embedding
- [ ] 切换模型后新文档使用新模型，旧文档向量不变（与 anything-llm 行为一致）
