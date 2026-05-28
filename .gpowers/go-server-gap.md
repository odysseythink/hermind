  这 24 小时内已经合上的口子

  ┌────────────────────────────────────┬───────────┬────────────────────────────────────────────────────────────────────────────┐
  │                维度                │ 5/26 状态 │                                  今天状态                                  │
  ├────────────────────────────────────┼───────────┼────────────────────────────────────────────────────────────────────────────┤
  │ MCP hypervisor                     │ 设计稿    │ ✅ 16 个文件，5 admin route + tool-call 全部就位                           │
  ├────────────────────────────────────┼───────────┼────────────────────────────────────────────────────────────────────────────┤
  │ Agent runtime（@agent + WS）       │ ❌        │ ✅ PR-AR-1/2/3 已合：WS + pantheon conversation+agent + tool registry + 4  │
  │                                    │ 完全缺失  │ 默认 skill；PR-AR-4/5 plan 已写                                            │
  ├────────────────────────────────────┼───────────┼────────────────────────────────────────────────────────────────────────────┤
  │ WebSocket 基建                     │ ❌ 无     │ ✅ gorilla/websocket v1.5.3 入库                                           │
  ├────────────────────────────────────┼───────────┼────────────────────────────────────────────────────────────────────────────┤
  │ 后台任务调度                       │ ❌ 无     │ ✅ internal/workers/ 接了 robfig/cron/v3，框架就绪                         │
  ├────────────────────────────────────┼───────────┼────────────────────────────────────────────────────────────────────────────┤
  │ pantheon                           │ ❌ 未导入 │ ✅ 6 处导入                                                                │
  │ conversation/agent/extensions      │           │                                                                            │
  └────────────────────────────────────┴───────────┴────────────────────────────────────────────────────────────────────────────┘

  仍未补上的硬缺口（按重要性）

  结构性（前端/集成会感知）：
  - ❌ endpoints/communityHub — 社区资源 + agent skill marketplace
  - ❌ endpoints/mobile/ — 移动端整片
  - ❌ endpoints/invite — 邀请流（多用户模式必需）
  - ❌ endpoints/agentSkillWhitelist — agent 工具批准白名单（PR-AR-5 plan 里明确延期到 Phase 2）
  - ❌ endpoints/workspacesParsedFiles + agentFileServer — agent 文件服务
  - ❌ TTS（elevenLabs/openAi/openAiGeneric）
  - ❌ webPush + PushNotifications/
  - ❌ PasswordRecovery
  - ❌ endpoints/experimental/{liveSync, imported-agent-plugins}

  能力面（pantheon 已经有、Go 接线没做完）：
  - ⚠️ providers/llm.go 只接了 2/43 个 provider（ollama + openai）
  - ⚠️ embedder/pantheon.go 只接了 openai
  - ⚠️ extensions/rerank 完全未导入（向量库的 Rerank bool 是 fetch-more-truncate，不是真 reranker）
  - ⚠️ agentFlows 投影有了，executor 没有（PR-AR-3 stub 返回 deferred）
  - ⚠️ imported plugins 是 v1 stub

  Agent skill 缺口（已在 plan 但未交付）：
  - ❌ sql-agent / filesystem / create-files — PR-AR-6
  - ❌ gmail / outlook / google-calendar OAuth — v3

  一句话

  主链路（chat / RAG / MCP / agent / 后台 worker）今天已经具备替代条件或在 PR-AR-4/5（约 17.5h
  工作量）之内完成。所有"边缘但用户可感知"的功能（mobile / communityHub / TTS / webPush / invite /
  passwordRecovery）一律未启动，这才是阻止"100%"的真正阻塞器。

  推荐下一步（仅一个，不分叉）

  先合 PR-AR-4 + PR-AR-5（约 2-3 工作日），把 @agent 主链路彻底闭环；然后把剩余 9 个 Node-only endpoint
  做一次"砍/留"决策——你能砍掉的（比如 experimental/liveSync 多半没人用），就在 GitHub issue 里记一条直接砍；要留的写进 P1 队列。

  要进入 PR-AR-4 执行就说一声，我直接进 gpowers:executing-plans 模式开干。