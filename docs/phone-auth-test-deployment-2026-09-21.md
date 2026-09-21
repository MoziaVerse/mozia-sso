# 测试服统一认证验收（2026-09-21，修正后的方案）

## 当前部署

- Casdoor：`mozia-sso:test-593bf9c6`，测试服 8778；保留原生 Casdoor 外观，撤掉 Matrix 视觉迁移。
- Matrix：`e86c3c6`，原登录页 4000、后端 3257；`CASDOOR_UNIFIED_AUTH=true`，前端启用测试服分端口配置。
- Matrix Casdoor 应用仅允许 `http://116.136.189.21:4000` 发起嵌入认证交接；原组织、client 和 sub 保持不变。
- Casdoor 文件会话持久化到 `~/app/mozia-sso/session-data:/tmp`，已有会话从原容器复制保留。
- 生产未部署、未改配置。

变更 PR：MoziaVerse/mozia-sso#4、MoziaVerse/matrix#218，均保持草稿。

## 本次真实浏览器验收

| 场景 | 结果 |
| --- | --- |
| Matrix 原页面短信登录 | 用户受控号码实际收到短信并提供验证码；在 Matrix 原表单完成认证，自动回到工作台 |
| 账号与业务数据 | 原账号继续使用；工作台、余额等正常读取；本次是真实老用户登录，不冒称真实新用户注册 |
| 浏览器 SSO | 带浏览器 cookie 请求 Casdoor userinfo 成功，sub 与 Matrix `/api/me` 一致；回跳临时状态已消费 |
| TTS Studio | 从 `/launch/tts-studio` 进入测试服 3260 工作台，交接 fragment 已消费，无再次验证码 |
| MoziaReel | 从 `/launch/mozia-reel` 进入测试服 1241 项目页，交接 fragment 已消费，无再次验证码 |
| Canvas | 从 `/launch/zeo-canvas` 进入测试服 13000 项目库，原账号与页面正常，交接 fragment 已消费 |
| 容器替换后会话 | 强制重建同一新版 Casdoor 容器后，浏览器仍能读取 userinfo，sub 与 Matrix 一致 |
| 回跳边界 | 测试服 Matrix 对与真实 Origin 不匹配的回跳地址返回 400；Casdoor 拒绝无效浏览器交接票据 |
| 原生托管页 | `/signup/mozia-matrix` 恢复原生 Casdoor 手机／账号表单；应用 Logo 使用已有 Matrix 图标资源 |

未点击任何生成、消费或付费功能。手机号、验证码、票据和具体 subject 不写入验收文档。

## 修复的测试配置偏差

测试 Matrix 原生产构建向 4000 的 `/api` 发请求，实际 404，后端在 3257。新增显式测试构建开关 `BUN_PUBLIC_SPLIT_ORIGIN_API=true`；生产默认同源行为不变。

测试 Canvas 进程沿用生产认证配置：`MATRIX_BACKEND_URL` 指向生产 Matrix，前端地址为生产域名，并使用 Secure cookie。导致测试票据在生产侧换票报 400。新增测试专用 `~/app/mozia-canvas/test-auth.env` 覆盖后端、前端、应用 origin 与 HTTP cookie 设置，保留原 `prod.env`；仅重启测试 Canvas，未改其代码或生产实例。

## 回滚

部署前备份位于测试服 `~/app/mozia-sso-backups/20260921-embedded-593bf9c6`，权限仅服务账号可读，包含 SSO 数据库、原 compose、Matrix 配置与 Canvas 原配置。

优先将 Matrix 后端 `CASDOOR_UNIFIED_AUTH=false` 并按原命令重启，旧认证路径仍可用。若回滚前端代码，测试分端口配置也需一起处理，不能恢复到已知 `/api` 404 的组合。SSO 镜像可回到 `test-040d106d`，但应保留新增的 `/tmp` 会话持久化卷；增量表无需删除。Canvas 回退只需移除测试覆盖，但会恢复已确认的跨环境握手错误。

## 尚未完成

- MoSpace 已有独立 Casdoor client，保持原组织与原跳转关系。其协议地址为空，已向用户询问；在可展示的协议明确前，暂未开启它的手机号一体注册开关。
- 真实新手机号、增长邀请码奖励、代理商首次自动归属、MFA 与第三方身份提供方仍未做真实端到端验收。新注册及禁用／并发等在一次性 PostgreSQL 和本机短信桩完成 HTTP 回归。
- 各产品退出会话仍独立，未改为全平台强制退出。

## SSO 主站组织入口补验

用户实际使用的地址是 `/login/mozia-internal`。此前只验证业务应用托管页，遗漏了这个组织入口。2026-09-21 已部署 `mozia-sso:test-ad13bf96`，对其既有内部登录配置启用手机号优先交互，保持 `enableSignUp=false`，不更改内部账号组织、角色或权限。

浏览器实测：同一地址刷新后默认“手机登录”，显示区号、手机号、验证码与“登录”按钮；没有自动注册提示或新增协议勾选；可切换“账号登录”，无横向溢出。一次性 PostgreSQL HTTP 回归验证关闭注册时已有账号无需注册协议即可登录，未知手机号仍不能建号，Matrix 及票据交接回归通过。本次未使用内部真实账号发送短信，因此不冒称完成内部账号真实短信验收。

部署前备份：`~/app/mozia-sso-backups/20260921-internal-phone`。回退页面开关可执行备份中的 `rollback.sql`；镜像回到 `test-593bf9c6` 时继续保留会话持久化卷。
