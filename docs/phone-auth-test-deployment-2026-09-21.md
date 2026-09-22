> 历史测试记录：本文描述 2026-09-21 的试验版本和测试服状态，不代表当前生产范围。当前实现以 [2026-09-22 生产记录](phone-login-production-2026-09-22.md) 为准。相关试验 PR 已关闭。

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

- 第一阶段曾因 MoSpace 协议地址为空暂缓；第二阶段已确认现有《统一账户与服务用户协议》覆盖全平台，完成启用，见下文。
- 真实新手机号、增长邀请码奖励、代理商首次自动归属、MFA 与第三方身份提供方仍未做真实端到端验收。新注册及禁用／并发等在一次性 PostgreSQL 和本机短信桩完成 HTTP 回归。
- 各产品退出会话仍独立，未改为全平台强制退出。

## SSO 主站组织入口补验

用户实际使用的地址是 `/login/mozia-internal`。此前只验证业务应用托管页，遗漏了这个组织入口。2026-09-21 已部署 `mozia-sso:test-ad13bf96`，对其既有内部登录配置启用手机号优先交互，保持 `enableSignUp=false`，不更改内部账号组织、角色或权限。

浏览器实测：同一地址刷新后默认“手机登录”，显示区号、手机号、验证码与“登录”按钮；没有自动注册提示或新增协议勾选；可切换“账号登录”，无横向溢出。一次性 PostgreSQL HTTP 回归验证关闭注册时已有账号无需注册协议即可登录，未知手机号仍不能建号，Matrix 及票据交接回归通过。本次未使用内部真实账号发送短信，因此不冒称完成内部账号真实短信验收。

部署前备份：`~/app/mozia-sso-backups/20260921-internal-phone`。回退页面开关可执行备份中的 `rollback.sql`；镜像回到 `test-593bf9c6` 时继续保留会话持久化卷。


## 第二阶段：公共主站及独立子应用（2026-09-21）

公共主站为 `/`、`/login`、`/signup`，使用 `mozia` 组织新建的 `mozia-account`，默认手机号验证码，统一“登录 / 注册”。`publicLoginApplication = "mozia-account"` 显式配置公共入口；未配置时保留 Casdoor 原默认行为。组织 `mozia.default_application` 从失效的旧名称修正到此公共账户应用。公共入口不再跟随浏览器的 `lastLoginOrg` 误入内部员工组织。`/login/mozia-internal` 保留内部账号登录，不开放内部注册；不是普通用户的主站链接。

MoSpace 保留原独立 client、组织及已有回调，仅开启手机号一体登录／注册，使用已存在且覆盖全系列应用的《统一账户与服务用户协议》。保留 Casdoor 原生视觉，未复制 Matrix 产品样式。其当前测试回调仍是本地/局域网地址，因此托管认证页验收不等于实际 MoSpace 部署的回跳验收。

新增应用 `mozia-canvas`、`mozia-tts-studio`、`mozia-reel`，各有独立生成的 client ID/secret，只允许 authorization_code，使用 JWT/RS256，开启严格回调匹配及自动 SSO。三个回调分别为测试 Matrix backend `/api/external/oidc/{zeo-canvas,tts-studio,mozia-reel}/callback`。`enableStrictRedirectUri` 默认关闭，旧应用行为不变；新应用开启后不允许任意 localhost、正则子串或附加路径。密钥只保存于受限服务器配置。

普通用户 `/apps` 中可见三个应用卡片，指向 Matrix 原 `/launch/<clientId>?sso=1`；认证由每个独立 Casdoor client 完成，Matrix BFF 保留业务交接、钱包及原 UI。MoSpace 不归入 Matrix 子应用。

本次代码：SSO `84ce5231`（镜像 `mozia-sso:test-84ce5231`），Matrix 后端包含独立回调与静默登录回退，详见 Matrix PR #218。备份位于 `~/app/mozia-sso-backups/20260921-public-apps`，含 SSO 数据库、两边配置及定向回滚 SQL；回滚不删除用户，不整库覆盖。

已完成：Go 定向测试；隔离 PostgreSQL + 本机短信桩的手机号新旧账号、策略、并发、浏览器会话、静默授权、严格回调和显式提供方回归；浏览器确认主站与 MoSpace 默认手机号登录／注册；测试服同一已有账号通过三个独立 client 授权并进入各应用，SSO token 记录确认三者属于同一用户。已清除 Matrix 会话后复用 SSO 会话进入 TTS Studio，无需再次验证码。

本轮未发送真实新号码短信；此前真实短信已通过 Matrix 原表单验证上游发码及浏览器 SSO。真实新账号业务奖励、代理商首次归属、MFA 全链路与全局退出不在本轮完成声明中。


补充实测：主站在记住 `lastLoginOrg=mozia-internal` 时仍进入公共手机号表单；`/signup` 使用同一登录／注册表单。实际点击 `/apps` 中 Canvas 卡片成功进入测试项目库。完全无 SSO 与 Matrix 会话的应用入口回到 Matrix 原登录页，保留应用回跳；未把普通访客留在认证库错误页。

## 普通用户登录被应用标签拒绝：配置纠正

用户真实手机登录出现“用户的标签不在该应用的标签列表中”。原因是新增应用配置误将 `application.tags` 当成展示分类：公共主站填了“统一账户”，三个子应用填了“Matrix”。Casdoor 的此字段实际是登录访问限制；普通用户没有相应标签会被 `HandleLoggedIn` 拒绝。注册开关一直开启，失败不是主站不支持注册。

测试服仅清空 `mozia-account`、`mozia-canvas`、`mozia-tts-studio`、`mozia-reel` 四个新增应用的错误标签；其他应用的权限策略、用户角色和生产配置未修改，无需重启。修改前标签保存在原受限备份目录的 `tags-before-fix.jsonl`。同步纠正一次性配置脚本及备份目录中的 `apply-applications.sql`，原脚本产物另存 `apply-applications-before-tag-fix.sql`，避免重放错误配置。

验收补充：隔离 PostgreSQL HTTP 测试新增公共主站 `type=login` 普通新账号注册、老账号登录和稳定 subject 检查；同一无管理员权限、无标签账号在设置限制后被拒绝，清空限制并使用新验证码后恢复登录。完整手机号 HTTP 回归通过。测试服公开应用接口已确认四个应用均无标签限制且手机注册开关开启。此前已有 SSO 会话的子应用验收未覆盖普通账号从主站首次认证，不能替代本次回归。修复后的真实短信浏览器验收仍需重新获取验证码完成，不将隔离短信桩结果宣称为真实短信验收。
