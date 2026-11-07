# WorkTools

一个基于 **Go + Wails v3** 的桌面运维工作台，把日常运维中分散的 URL、口令、OTP、笔记和常用小工具收敛到一个本地加密的桌面应用里。

## 功能模块

| 模块 | 说明 |
| --- | --- |
| **常用 URL** | 保存常用系统的地址、账号密码、环境与标签，支持一键打开并自动填写登录表单、一键复制密码（30 秒后自动清理剪贴板） |
| **密码保险箱** | 个人重要口令（服务器 / 数据库 / 云平台 / 其他），可保存密码与附加信息（私钥、AK/SK 等） |
| **OTP 验证码** | TOTP / HOTP，支持 `otpauth://` URI、二维码图片导入，以及 Google Authenticator 批量迁移格式（`otpauth-migration://`）的导入与导出，列表中实时显示验证码与倒计时 |
| **记事本** | Markdown 编辑 + 实时预览，支持多条记录、置顶，可导出 Markdown / HTML / PDF |
| **私密文件夹** | 加密保存身份证复印件、营业执照等重要文件；小文件密文存入数据库、大文件加密落盘，支持自定义分类、备注、图片 / PDF 预览与导出 |
| **运维工具箱** | 见下表 |
| **备份与恢复** | 全量数据导出为加密备份文件，可随时还原；支持设置密保问题、通过密保答案重置主密码 |

### 运维工具箱

| 分类 | 工具 |
| --- | --- |
| 文本处理 | 行转列 / 列转行、多列拼接、加单双引号、去重排序、文本 ↔ JSON 互转、自定义前后缀拼接（如 `http://` + 内容 + `.net`） |
| 编码/哈希 | Base64 / Hex 编解码、MD5 / SHA1 / SHA256 |
| JSON | 格式化 / 压缩 |
| 时间 | Unix 时间戳 ↔ RFC3339 互转 |
| 网络 | SSL 证书检测（评级 A–F、协议支持、证书链、剩余有效期）、端口探测、HTTP 探测、DNS 查询、子网计算 |
| 密码生成 | 复杂度档位（低/中/高）+ 自定义字符类与符号集、排除易混淆字符，生成后可加备注存入密码保险箱 |
| 随机数据 | 随机邮箱、随机手机号、随机身份证号（校验码合法，仅供测试）、随机串 |
| 图片处理 | 图片压缩：设定目标大小 / 最长边 / 质量，自动选择格式并二分搜索 JPEG 质量以满足报名等场景的大小限制，支持批量选择与输出目录 |

## 安全设计

主密码不落盘，全部敏感字段在写库前完成加密：

```
主密码 ──Argon2id(time=3, mem=64MiB, threads=4)──▶ 主密钥(32B)
主密钥 ──AES-256-GCM──▶ 包裹随机 DEK
DEK    ──AES-256-GCM──▶ 逐字段加密（表名:列名 作为 AAD 绑定）
```

- 每个字段使用独立随机 nonce；AAD 绑定逻辑位置，密文无法在字段间搬运。
- 锁定后内存中的密钥立即清零，所有读写接口返回 `vault is locked`。
- 密保答案仅存哈希，明文不落盘；重置密码时在 Go 侧校验。
- 数据仅保存在本机用户目录，不上传任何服务器。

## 目录结构

```
.
├── main.go                     # Wails 入口，注册服务、内嵌前端产物
├── docs/PLAN.md                # 规划与设计文档
├── internal/
│   ├── config/                 # 按用户解析数据目录
│   ├── store/                  # SQLite 打开、schema、领域模型
│   ├── vault/                  # KDF / AEAD / 保险箱生命周期 / 加密 CRUD / 密保恢复
│   ├── otp/                    # TOTP / HOTP 生成、otpauth:// 与迁移格式解析/导出、二维码识别
│   └── toolbox/                # 无状态运维工具（网络探测、文本处理、密码生成、随机数据）
├── services/                   # 面向前端绑定的服务层
└── frontend/                   # Vue3 + TS + Vite + Naive UI
    ├── src/views/              # Unlock / Urls / Secrets / Otp / Notes / Files / Toolbox / Settings
    ├── src/components/         # 通用组件
    ├── src/lib/                # 绑定封装、共享状态
    └── bindings/               # wails3 生成的前端绑定
```

## 数据存储位置

数据库位于各平台的用户配置目录下：

| 平台 | 路径 |
| --- | --- |
| macOS | `~/Library/Application Support/notebook/notebook.db` |
| Windows | `%AppData%\notebook\notebook.db` |
| Linux | `~/.config/notebook/notebook.db` |

> 目录名沿用 `notebook`（历史数据兼容）。应用名已改为 WorkTools，已有数据库无需迁移。

## 开发与构建

前置要求：Go ≥ 1.26、Node.js ≥ 22.12、[wails3 CLI](https://v3.wails.io/)。

```bash
# 安装/更新 wails3 CLI
go install github.com/wailsapp/wails/v3/cmd/wails3@latest

# 开发模式（前后端热重载）
wails3 dev

# 重新生成前端绑定（改动 services/ 后；-ts 必须带，否则前端导入断裂白屏）
wails3 generate bindings -d frontend/bindings -ts

# 生产构建
wails3 build
```

> 本机环境设置了 `GOSUMDB=off`，会阻断 Go 模块校验；执行任何 `go` / `wails3` 模块解析命令时请显式带上 `GOSUMDB=sum.golang.org`。

## 测试

```bash
go test ./internal/...
```

覆盖保险箱初始化 / 解锁 / 锁定、错误口令、短口令拒绝、密保重置、URL 与 OTP 加解密往返、锁定态访问保护、OTP 迁移格式往返（含 base32 填充兼容）、文本转换往返、密码生成复杂度保证、随机身份数据格式与身份证校验码、SSL 检测冒烟。

## 规划文档

详见 [docs/PLAN.md](docs/PLAN.md)。
