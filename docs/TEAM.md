# WorkTools 多角色协作手册

> 本仓库通过 `.github/agents/` 下的自定义角色 + `docs/` 下的文档流实现多会话/多 Agent 协作。
> 所有角色文件见 `.github/agents/*.agent.md`,可在聊天面板的 Agent 选择器中切换,或由主会话作为子 Agent 派发。

## 角色总览

| 角色 | 文件 | 职责 | 写权限 |
|------|------|------|--------|
| 架构师 | `architect.agent.md` | 需求拆解、接口契约、任务包分发 | `docs/tasks/`、`docs/decisions/`、`docs/PLAN.md` |
| 产品经理 | `pm.agent.md` | PRD、用户故事、验收标准 | `docs/prd/` |
| 前端工程师 | `frontend.agent.md` | Vue3 页面/组件/类型 | `frontend/src/**` |
| 后端工程师 | `backend.agent.md` | Go 领域逻辑/绑定/存储 | `internal/**`、`services/**` |
| 测试工程师 | `tester.agent.md` | 单测、验收测试、测试报告 | `**/*_test.go`、`docs/test-reports/` |
| 用户代表 | `user.agent.md` | 模拟验收、体验反馈 | `docs/feedback/` |

## 协作流程

```
用户
 │  提出想法
 ▼
产品经理 ──► docs/prd/PRD-xxx.md(用户故事 + 验收标准)
 │
 ▼
架构师 ──► docs/tasks/T-xxx.md(契约 + 文件领地 + depends)
 │           ├── 无依赖的任务包 ──► 可并行分发
 ▼
前端工程师 ─┐
后端工程师 ─┤   并行执行,各自只碰自己的领地
            │
 ▼
测试工程师 ──► docs/test-reports/T-xxx.md(PASS/FAIL + bug 清单)
 │
 ▼
用户代表 ──► docs/feedback/F-xxx.md(体验问题)
 │
 ▼
产品经理分诊 → 回到对应角色修复 → 循环直到验收通过
```

## 并行执行规则(多会话协作核心)

1. **文件领地互斥**:每个任务包的 `files` 清单不得与其他进行中的任务包重叠。这是硬性要求,架构师拆分时保证,执行者发现冲突立即上报。
2. **热点文件收口**:`main.go`、`go.mod`、`package.json`、`frontend/src/router/`、`Taskfile.yml` 由主会话统一修改,角色不得私自改动。
3. **依赖串行**:`depends` 标注的任务必须等前置完成;未标注默认可并行。
4. **依赖安装集中**:任何角色需要新依赖时,在输出中"上报",由主会话统一 `go get` / `npm install`,避免锁文件冲突。
5. **文档即通信**:会话之间不能对话,一切状态落在 `docs/` 文件里。新会话接手前先读 `docs/TEAM.md` + 对应任务包。

## 同角色多会话并行(如:多个后端开发)

角色文件是**岗位说明书,不是单例**——同一个"后端工程师"角色可以被多个会话同时使用。
并行度不来自"克隆角色",而来自**任务包的领地划分**。后端领地内部有天然分界线:

### 后端域地图(架构师拆任务时按"域"切,一个域 = 一个可并行单元)

| 域 | internal 包 | 绑定的 service 文件 | 说明 |
|----|-------------|---------------------|------|
| OTP | `internal/otp/` | `services/otp_service.go` | 独立,可独占 |
| 日程/节假日 | `internal/holiday/` | `services/schedule_service.go` | 独立,可独占 |
| 截图/OCR | `internal/screenshot/` | `services/screenshot_service.go` | 注意也依赖 `internal/config` |
| 剪贴板 | `internal/clipboard/` | `services/clipboard_service.go` | 独立,可独占 |
| 工具箱 | `internal/toolbox/` | `services/toolbox_service.go` + `services/workbench_service.go` | ⚠️ 两个 service 都依赖 toolbox,必须捆绑给同一人 |
| 备份/云同步 | `internal/cloudsync/` + `internal/settings/` | `services/backup_service.go` | 捆绑 |
| 保险箱核心 | `internal/vault/` | `services/vault_service.go` 等 | 🔴 共享热点,见下 |

### 后端内部热点文件(多后端并行的真正风险点)

以下文件被**几乎所有域**依赖,多后端同时改必然冲突:

- `internal/store/models.go`、`internal/store/schema.sql`、`internal/store/store.go`
- `internal/vault/vault.go`、`internal/vault/crypto.go`、`internal/vault/repo.go`

**规则**:
1. 架构师拆任务时,凡涉及上述热点文件的需求,**单独拆成一个"store/vault 契约任务"并串行**(depends 排在最前),由**一个**后端会话完成。
2. 契约任务完成、`models.go`/`schema.sql` 定稿后,各域任务才能并行开工——各域只**读**契约,不再改它。
3. 域任务执行中发现契约不够用:**上报架构师**,由架构师决定是排期第二个契约任务还是给该域开特例,禁止域任务私自改 `models.go`。

### 认领协议(避免两个后端抢同一个任务)

**指派优先于认领**:架构师拆任务时直接在任务包里写 `owner: 后端A` / `owner: 后端B`,各会话只做指派给自己的任务——这从根上消灭竞争。

若必须自由认领,协议为:
1. 读任务包,确认 `status: pending`
2. 原子化认领:把 `status: pending` 改为 `status: in-progress`,同时更新 `docs/tasks/README.md` 索引表
3. 改完立刻 `git add + commit` 该任务包文件,commit 即"锁";后提交者 rebase 时发现冲突须让路
4. 认领后 24 小时无进展,其他会话可接手(把原 owner 注明在 notes 里)

## 目录约定

```
docs/
├── TEAM.md           # 本手册
├── PLAN.md           # 项目总体规划
├── prd/              # PRD-<编号>-<slug>.md
├── tasks/            # T-<编号>-<slug>.md(任务包,含领地与验收)
├── decisions/        # ADR 技术决策记录
├── test-reports/     # 测试报告
└── feedback/         # 用户反馈
```

## 典型用法

### 用法一:单会话流水线(推荐入门)

在一个会话里按顺序切换角色:

```
你: /pm 把 M5 工具箱加一个"JSON 格式化"工具
   → 产出 PRD
你: /architect 按 PRD-xxx 拆任务
   → 产出 T-001(后端)、T-002(前端)
你: /backend 执行 T-001
你: /frontend 执行 T-002
你: /tester 给 T-001 写测试并验收
```

### 用法二:多会话并行(效率优先)

主会话让架构师拆好任务包后,开多个 VS Code 窗口/会话:

- 窗口 A(后端会话):`执行 docs/tasks/T-001-xxx.md`
- 窗口 B(前端会话):`执行 docs/tasks/T-002-xxx.md`
- 窗口 C(测试会话):只写测试,等实现完成再跑

**前提**:任务包领地不重叠;`main.go` 等热点文件留给主会话收口。

### 用法三:子 Agent 派发(单会话内并行)

主会话直接说:"用测试工程师角色给 internal/otp 写覆盖测试"——主会话会以子 Agent 形式派发,测试角色只读实现、只写 `*_test.go`,与写代码的主线互不干扰。

## 冲突处理

- 两个任务包领地重叠 → 找架构师重新拆分,后启动的任务让路
- 契约需要变更 → 架构师更新任务包,通知所有依赖方;已完成的实现按新契约返工
- 测试发现 bug → 测试工程师写复现测试 + 报告,不修实现;修复由对应实现角色领任务
