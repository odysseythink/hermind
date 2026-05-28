# backend Vector DB 完整支持 — 设计文档

> 目标：将 Node.js server 中所有 Vector DB provider 移植到 backend。
> 日期：2026-05-26
> 状态：已批准

---

## 1. 架构调整

### 1.1 接口变更

`VectorDatabase` 接口的 `DeleteVectors` 签名从接收 `docIds` 改为接收 `vectorIds`：

```go
type VectorDatabase interface {
    Name() string
    Connect(ctx context.Context) error
    Heartbeat(ctx context.Context) (map[string]any, error)
    AddVectors(ctx context.Context, namespace string, chunks []VectorChunk) error
-   DeleteVectors(ctx context.Context, namespace string, docIds []string) error
+   DeleteVectors(ctx context.Context, namespace string, vectorIds []string) error
    SimilaritySearch(ctx context.Context, namespace string, queryVector []float32, opts SearchOptions) ([]SearchResult, error)
    DeleteNamespace(ctx context.Context, namespace string) error
    Tables(ctx context.Context) ([]string, error)
    TotalVectors(ctx context.Context) (int64, error)
}
```

### 1.2 DocumentService 层统一转换

`UpdateEmbeddings` 和 `RemoveAndUnembed` 中，删除前先查 `DocumentVectors` 表获取 `vectorIds`：

```go
var docVectors []models.DocumentVector
db.Where("doc_id IN ?", docIds).Find(&docVectors)
vectorIds := extractVectorIds(docVectors)
vectorDB.DeleteVectors(ctx, wsSlug, vectorIds)
```

### 1.3 Provider 工厂

`VectorService.Connect` 根据 `cfg.VectorDB` 创建对应的 provider：

```go
switch s.cfg.VectorDB {
case "lancedb":
    s.provider = vectordb.NewLanceDB(s.cfg.StorageDir)
case "pgvector":
    s.provider = vectordb.NewPGVector(s.cfg.DatabaseURL)
case "pinecone":
    s.provider = vectordb.NewPinecone(s.cfg.PineconeAPIKey, s.cfg.PineconeIndex)
case "qdrant":
    s.provider = vectordb.NewQdrant(s.cfg.QdrantEndpoint, s.cfg.QdrantAPIKey)
case "chroma":
    s.provider = vectordb.NewChroma(s.cfg.ChromaEndpoint, s.cfg.ChromaAPIHeader, s.cfg.ChromaAPIKey)
case "weaviate":
    s.provider = vectordb.NewWeaviate(s.cfg.WeaviateEndpoint, s.cfg.WeaviateAPIKey)
case "milvus":
    s.provider = vectordb.NewMilvus(s.cfg.MilvusAddress, s.cfg.MilvusUsername, s.cfg.MilvusPassword)
case "zilliz":
    s.provider = vectordb.NewZilliz(s.cfg.ZillizEndpoint, s.cfg.ZillizAPIToken)
case "astra":
    s.provider = vectordb.NewAstraDB(s.cfg.AstraDBApplicationToken, s.cfg.AstraDBEndpoint)
case "chromacloud":
    s.provider = vectordb.NewChromaCloud(s.cfg.ChromaEndpoint, s.cfg.ChromaAPIKey)
default:
    return fmt.Errorf("unknown vector db: %s", s.cfg.VectorDB)
}
```

---

## 2. 第一批：LanceDB 完整实现

### 2.1 Schema 设计

每个 namespace 对应一张 LanceDB table，schema：

| 字段 | 类型 | 说明 |
|-----|------|------|
| `id` | utf8 (string) | vector chunk UUID |
| `vector` | fixed_size_list(float32, N) | 嵌入向量 |
| `doc_id` | utf8 | 关联文档 ID |
| `text` | utf8 | 原始文本片段 |
| `metadata` | utf8 (JSON string) | 其他元数据 |

### 2.2 核心方法

使用 `github.com/lancedb/lancedb-go` SDK：

- **Connect**: `lancedb.Connect(ctx, uri, nil)`
- **AddVectors**: 打开/创建表 → `chunksToRecord` → `table.Add(ctx, record, nil)`
- **SimilaritySearch**: `table.VectorSearch(ctx, "vector", queryVector, topN)`，cosine 距离 → 相似度转换
- **DeleteVectors**: `table.Delete(ctx, "id IN ('xxx','yyy')")`
- **DeleteNamespace**: `conn.DropTable(ctx, namespace)`
- **Tables**: `conn.TableNames(ctx)`
- **TotalVectors**: 遍历所有表 `table.Count(ctx)` 求和

### 2.3 辅助函数

- `chunksToRecord`: `[]VectorChunk` → Arrow Record（使用 arrow Array builder）
- `parseSearchResults`: Arrow Record → `[]SearchResult`
- `openOrCreateTable`: 检查表是否存在，不存在则创建 schema 并建 IVF-PQ 索引

---

## 3. 第二批：常用云 Provider（Pinecone / Qdrant / Chroma）

### 3.1 Pinecone

| 项目 | 设计 |
|-----|------|
| SDK | `github.com/pinecone-io/go-pinecone` |
| 连接 | `pinecone.NewClient(pinecone.NewClientParams{ApiKey: cfg})` → `client.Index(indexName)` |
| Namespace | Pinecone namespace（直接映射） |
| AddVectors | `index.Namespace(namespace).UpsertVectors([]Vector{...})`，metadata 包含 `docId`/`text` |
| SimilaritySearch | `index.Namespace(namespace).Query(vector, topK, includeMetadata)` |
| DeleteVectors | `index.Namespace(namespace).DeleteVectorsById(vectorIds)` |
| Tables | `client.DescribeIndex().Namespaces` 的 keys |

### 3.2 Qdrant

| 项目 | 设计 |
|-----|------|
| SDK | `github.com/qdrant/go-client` (gRPC) |
| 连接 | `qdrant.NewClient(qdrant.Config{Host, Port, APIKey})` |
| Namespace | Qdrant collection name（直接映射） |
| AddVectors | 先 `CreateCollection`（维度从首 chunk 推断，距离=Cosine），再 `Upsert` points |
| SimilaritySearch | `client.QueryPoints(collection, vector, limit, withPayload)` |
| DeleteVectors | `client.DeletePoints(collection, vectorIds)` |
| 特殊处理 | 创建 collection 时需要指定向量维度和距离度量 |

### 3.3 Chroma

| 项目 | 设计 |
|-----|------|
| SDK | `github.com/amikos-tech/chroma-go` |
| 连接 | `chromago.NewClient(chromaUrl)`，可选 auth header |
| Namespace | Chroma collection name（**需要 normalize**，3-63字符，alphanumeric/underscore/hyphen） |
| AddVectors | `client.GetOrCreateCollection(name, metadata{"hnsw:space": "cosine"})` → `collection.Add(ids, embeddings, metadatas, documents)` |
| SimilaritySearch | `collection.Query(queryEmbeddings, nResults)`，distance → similarity 转换 |
| DeleteVectors | `collection.Delete(ids)` |
| 特殊处理 | `normalize()` 函数确保 collection 名符合 Chroma 规则 |

---

## 4. 第三批：剩余 Provider（Milvus / Weaviate / Astra / Zilliz / ChromaCloud）

### 4.1 Milvus

| 项目 | 设计 |
|-----|------|
| SDK | `github.com/milvus-io/milvus-sdk-go/v2` |
| 连接 | `client.NewClient(ctx, milvus.Config{Address, Username, Password})` |
| Namespace | collection name（normalize：字母/数字/下划线，`anythingllm_` 前缀） |
| AddVectors | `CreateCollection`（id varchar PK, vector floatVector, metadata JSON）→ `CreateIndex`（AUTOINDEX + COSINE）→ `LoadCollection` → `Insert` |
| SimilaritySearch | `client.Search(ctx, collection, vectors, metricType, topK, sp)` |
| DeleteVectors | `client.DeleteByPks(ctx, collection, partition, vectorIds)` |
| 特殊处理 | 插入后需要 `Flush` 才能正确统计数量 |

### 4.2 Weaviate

| 项目 | 设计 |
|-----|------|
| SDK | `github.com/weaviate/weaviate-go-client/v4` |
| 连接 | `weaviate.New(weaviate.Config{Host, Scheme, AuthConfig})` |
| Namespace | Weaviate class name（camelCase：如 `myWorkspace` → `MyWorkspace`） |
| AddVectors | `Schema().ClassCreator().WithClass(...)`（vectorizer: "none"）→ `Batch().ObjectsBatcher().WithObjects(...)` |
| SimilaritySearch | GraphQL: `client.GraphQL().Get().WithClassName(camelCase).WithFields(...).WithNearVector(...).WithLimit(topN)` |
| DeleteVectors | `client.Data().Deleter().WithClassName(camelCase).WithId(vectorId)` |
| 特殊处理 | metadata 需要 `flattenObjectForWeaviate`（不支持嵌套对象） |

### 4.3 Astra DB

| 项目 | 设计 |
|-----|------|
| SDK | **无稳定 Go SDK** → 直接用 `net/http` 调用 Data API |
| 连接 | endpoint + `Token` header |
| Namespace | collection name（sanitize：`ns_` 前缀，替换非法字符为 `_`） |
| AddVectors | POST `/v1/{namespace}` → `insertMany`（batch size ≤20，Astra 限制） |
| SimilaritySearch | POST `/v1/{namespace}` → `find` with `$vector` sort + `includeSimilarity` |
| DeleteVectors | POST `/v1/{namespace}` → `deleteMany` with `_id` filter |
| 特殊处理 | collection 创建需要指定 `dimension` 和 `metric: cosine` |

### 4.4 Zilliz & ChromaCloud（继承模式）

```go
// Zilliz 嵌入 Milvus，只覆盖 Connect
type Zilliz struct { *Milvus }
func (z *Zilliz) Connect(ctx context.Context) error {
    client, err := milvus.NewClient(ctx, milvus.Config{
        Address: z.cfg.ZillizEndpoint,
    })
    // ...
}

// ChromaCloud 嵌入 Chroma，只覆盖 Connect
type ChromaCloud struct { *Chroma }
```

---

## 5. 配置变更

`backend/internal/config/config.go` 新增字段：

```go
// === Pinecone ===
PineconeAPIKey string `env:"PINECONE_API_KEY"`
PineconeIndex  string `env:"PINECONE_INDEX"`

// === Qdrant ===
QdrantEndpoint string `env:"QDRANT_ENDPOINT"`
QdrantAPIKey   string `env:"QDRANT_API_KEY"`

// === Chroma / ChromaCloud ===
ChromaEndpoint  string `env:"CHROMA_ENDPOINT"`
ChromaAPIHeader string `env:"CHROMA_API_HEADER"`
ChromaAPIKey    string `env:"CHROMA_API_KEY"`

// === Weaviate ===
WeaviateEndpoint string `env:"WEAVIATE_ENDPOINT"`
WeaviateAPIKey   string `env:"WEAVIATE_API_KEY"`

// === Milvus ===
MilvusAddress  string `env:"MILVUS_ADDRESS"`
MilvusUsername string `env:"MILVUS_USERNAME"`
MilvusPassword string `env:"MILVUS_PASSWORD"`

// === Zilliz ===
ZillizEndpoint string `env:"ZILLIZ_ENDPOINT"`
ZillizAPIToken string `env:"ZILLIZ_API_TOKEN"`

// === Astra DB ===
AstraDBApplicationToken string `env:"ASTRA_DB_APPLICATION_TOKEN"`
AstraDBEndpoint         string `env:"ASTRA_DB_ENDPOINT"`
```

同时 `.env.example` 文件同步更新示例值。

---

## 6. 测试策略

### 6.1 单元测试

每个 provider 测试：
- `Connect` — mock HTTP/gRPC，验证请求格式
- `AddVectors` — 验证数据转换和批处理
- `SimilaritySearch` — 验证结果解析和相似度转换
- `DeleteVectors` — 验证删除请求参数
- `Tables` / `TotalVectors` — 验证统计信息

工具：`testify/mock` + `httptest`

### 6.2 集成测试

外部服务可用时运行，默认跳过：

```go
func TestPineconeVectorDB(t *testing.T) {
    if os.Getenv("PINECONE_API_KEY") == "" {
        t.Skip("PINECONE_API_KEY not set")
    }
    // ...
}
```

### 6.3 LanceDB 特殊处理

LanceDB 是本地文件数据库，可完整测试：
- 创建临时目录
- 完整生命周期：Connect → Add → Search → Delete → DeleteNamespace

---

## 7. 分批实施计划

| 批次 | Provider | 关键工作 |
|------|---------|---------|
| **第一批** | LanceDB | 接口调整 + PGVector 适配 + LanceDB 完整实现 |
| **第二批** | Pinecone, Qdrant, Chroma | 3 个 REST/gRPC provider + 配置扩展 |
| **第三批** | Milvus, Weaviate, Astra, Zilliz, ChromaCloud | 5 个 provider（2 个继承模式） |
