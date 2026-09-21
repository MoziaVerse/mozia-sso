# 手机号登录／注册一体

该功能把 Matrix 的手机号一体认证迁入 Casdoor，默认关闭。本次实现 Casdoor 入口与验证基础；尚未切换 Matrix、Canvas、TTS、MoziaReel 的生产登录入口。

## 启用条件

- 当前实现针对 PostgreSQL。号码锁使用事务级 advisory lock，多进程共享；其他数据库启用该模式时请求会明确失败。连接池需要允许至少两条连接（锁连接与原有业务服务连接）。
- 在应用设置中开启“手机号登录／注册一体”（`enablePhoneSigninSignup`），并保留手机号 `Verification code` 登录方式。
- 用户组织不能是 `built-in`。新用户是否可创建仍由 `enableSignUp` 控制；关闭注册不影响已有用户登录。
- 配置短信供应商、手机号区域、用户协议内容。`termsOfUse` 应提供可访问的正式协议；上线前核对隐私政策是否包含在其中。
- 保留原组织、用户及 sub。用户名、密码不需要用户输入，由服务端生成；注册的其他必填项和 Casdoor invitation 要求仍按原应用配置检查。若应用要求必填邮箱或邀请码，需要先明确该应用是否适合此手机号模式，不能直接跳过这些要求。
- 启用 `enableSigninSession` 让浏览器保留 Casdoor Session。是否启用 `enableAutoSignin` 单独决定；各应用仍使用各自 OIDC client 和精确回调地址。
- schema 由现有 `CreateTables`／Xorm Sync2 增量添加 Application 开关及 `verification_record.failed_attempts`（默认 0）；不新建用户表、不重建身份。部署前先在测试库核对迁移。

## 页面行为

启用应用默认显示手机登录，手机号＋验证码完成登录／注册，不要求用户填写用户名和密码。账号密码与原第三方入口保留；手机号页隐藏独立注册和找回密码链接，找回密码仍在账号页。旧 `/signup` 页面在已启用应用下渲染同一手机登录组件，并保留 OAuth 参数；旧 `/api/signup` 保持兼容。

关闭 `enableSignUp` 时只显示“登录”，不提示自动注册。协议沿用 Casdoor 显式勾选，未同意不提交；目前尚未增加独立协议版本留痕机制，生产前需确定记录契约。增长邀请码仍归 Matrix，本页面不加入业务奖励字段。

## 请求契约

发码沿用 `POST /api/send-verification-code`，`type=phone`、`method=login`。启用该模式的应用允许为未知手机号发码，但关闭注册时仍拒绝未知手机号。原短信供应商、captcha 和号码验证继续使用。

认证沿用 `POST /api/login`。Casdoor 页面在手机号验证码方式下增加 `phoneSigninSignup: true` 和 `agreement: true`。服务端从应用确定组织，拒绝组织替换。新用户经原注册校验、默认属性、建用户服务，再进入 MFA／`HandleLoggedIn`；已有用户经原错误次数、MFA 和授权检查。OAuth 参数（state、nonce、PKCE、回调等）仍走既有 Casdoor 授权链路。

旧 Matrix 未携带 `phoneSigninSignup` 的请求仍按原登录逻辑处理，旧 `/signup` 继续可用。不要在 Matrix 迁移前删除旧接口，也不要把增长邀请码当成 Casdoor invitationCode。

## 频控与并发

启用模式后，号码级发送限制为同组织、规范化号码 60 秒冷却／滚动 24 小时 10 条成功发送记录，跨应用共享。该规则替代该应用发送路径的原 IP 冷却（生产原先因共享出口关闭了 IP 冷却）。限流返回 429 和 Casdoor 错误 envelope，`data.retryAfterSeconds` 提供剩余冷却秒数；0 表示没有短期重试时间。

发码、已启用应用的手机号登录与注册使用同一号码数据库锁。只认可当前组织该号码最新的验证码，使用条件更新消费，不回退到旧的未用验证码；错误尝试最多 5 次并写入数据库。已有用户还受原有账号锁定限制。

锁覆盖启用该模式应用的相关路径，不宣称能约束其他未启用应用、管理员建用户、其他绑定手机号入口。上线前检查同组织其他公开注册入口和既有重复手机号；不在未经对账的用户表上直接追加全局手机号唯一索引。

验证码在账户创建及后续授权之前消费。若后续数据库／外部同步失败，需要重新获取验证码；这是失败后拒绝复用的明确行为，不是跨所有副作用的大事务。新身份的注册结果如何供 Matrix 判断增长邀请资格，仍需在 Matrix 接入阶段完成。

## 本地验证

使用仓库 `go.mod` 指定的 Go 工具链、Yarn 1 安装前端依赖。

```sh
go test ./internal/phoneauth
go test ./object ./controllers -run 'TestPhone|TestIsAllowSend|TestHduBinding' -count=1
go build .
cd web
yarn install --frozen-lockfile
yarn test --watchAll=false --runInBand src/auth/phoneSignin.test.js
yarn build
```

`TestPhonePostgresLockAcrossConnections` 需通过 `PHONE_AUTH_TEST_POSTGRES` 指定一次性 PostgreSQL 测试库。不配置时跳过该项；其他验证码数据库测试使用临时 SQLite 文件。

完整 HTTP 冒烟脚本为 `scripts/test-phone-signin.py`：先启动使用一次性 PostgreSQL 数据库 `phone_auth_test` 的本地 Casdoor，完成 built-in 数据初始化；设置 `PHONE_AUTH_TEST_POSTGRES` 为该测试库连接串、`PHONE_AUTH_TEST_BASE_URL` 为本地地址（默认 `http://127.0.0.1:21878`），确保 PATH 有 `psql`，再运行：

```sh
python3 scripts/test-phone-signin.py
```

脚本强制检查数据库名及本地 HTTP 地址，生成独立测试组织／应用，写入合成验证码，并使用仅监听本机的 HTTP 短信桩验证实际发码接口，绝不向真实短信供应商发请求。覆盖新老用户 sub 一致、PKCE 换 token、nonce、Session、重复验证码、未同意协议、禁用注册、组织替换、并发建号、旧注册／密码登录和浏览器会话继续授权。测试数据留在一次性库中，完成后销毁该测试库。

2026-09-21 的验证范围和结果见 [验收记录](phone-auth-acceptance-2026-09-21.md)。真实短信供应商投递、生产配置、Matrix 实际回调及增长／代理商业务衔接不属于已通过项。
