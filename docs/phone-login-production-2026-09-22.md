# 生产 Casdoor 手机号一体登录

## 变更范围

本次按用户要求发布 Casdoor 公共手机号登录／注册页，保留摩智视界顶部 Logo，使用“统一账号”标识区分业务产品。表单对齐 Matrix 的分段标签、字段标签、圆角控件、间距和手机适配。账号密码入口保留。

代码版本：`d3196bd8`。运行镜像：`mozia-sso:prod-d3196bd8`。生产源码目录仍保留原状态；发布源码快照及构建产物位于生产 `~/app/mozia-sso-releases/20260922-d3196bd8`。

仅 `admin/mozia-matrix` 开启 `enablePhoneSigninSignup`，组织保持 `Mozia`，沿用原注册策略、短信供应商、用户协议、Logo、client 及回调地址。`publicLoginApplication=mozia-matrix` 将公共 `/`、`/login`、`/signup` 指向同一入口；组织 `/login/Mozia` 及原 Matrix OIDC 授权页复用该表单。未开放内部组织注册，其他应用开关未调整。

Matrix 与子应用的业务代码、入口、网关和 API Key 未变更。已有手机号继续使用原身份，新手机号验码后经原注册及授权流程创建身份。短信按号码冷却、限制错误次数，验证码单次使用。

## 本次验证

- Go 定向测试、号码策略测试、PostgreSQL 多连接号码锁测试通过。
- 一次性 PostgreSQL + 本机短信模拟服务 HTTP 回归通过：新老号码、身份连续性、并发单次建号、错误／重复验证码、禁用策略、关闭注册、原密码登录、非法回调、PKCE、nonce、state、SSO 后续授权。
- 前端手机号逻辑 4 项测试、ESLint、生产构建、diff 检查通过。
- 隔离环境浏览器确认手机默认入口、账号密码切换、Logo 及协议展示。390px 视口页面宽度等于 390px，无横向溢出。
- 生产只读查询未发现同组织下重复原始手机号；这不等同于所有历史号码格式均已人工对账。
- 生产浏览器确认公共 `/login` 已显示手机登录／注册、原 Logo、协议与第三方入口，账号密码可切换；携带原 Matrix client 和正式 callback 的 OIDC 授权页同样显示新表单。原回调数据库值未修改。
- 2026-09-22 11:40（UTC+8），用户完成真实手机验码操作；生产记录显示 11:40:37 发码、11:40:45 提交 `phoneSigninSignup=true`、`signinMethod=Verification code`、`type=login`，浏览器已进入 `/apps` 应用列表。确认生产手机号登录及 Casdoor 会话建立通过。
- 本次实际为公共主站登录，不是 Matrix OIDC 回调；新号码注册及真实 Matrix 授权回跳仍待专项验收，不能用本次主站登录或模拟短信结果替代。

## 运行与回退

使用原 `docker-compose.mozia.yml` 加 `docker-compose.phone-login.yml` 启动 SSO。原文件会话复制到 `session-data` 并挂载 `/tmp`，后续重建须继续保留该卷。

生产受限备份位于 `~/app/mozia-sso-backups/20260922-phone-login`，包括发布前数据库、配置、旧镜像标识及 `rollback.sh`。回退脚本恢复旧配置和旧镜像，保留会话卷及已创建用户，不覆盖用户数据库。新增列兼容旧版。该脚本已准备，本次未为了演练再中断生产。

## MoSpace 授权入口补齐

用户反馈 MoSpace 仍显示旧版。原因是上一轮仅开启 `mozia-matrix`，MoSpace 使用独立应用配置，遗漏了其应用级开关。

本轮为生产 `admin/MoSpace` 开启 `enablePhoneSigninSignup`，补充已有《统一账户与服务用户协议》地址（正文明确覆盖全系列服务）。保留原 Mozia 组织、Logo、client、注册策略与全部回调地址，无服务重启。修改前两项配置及用于核对的非秘密配置保存在同一受限备份目录 `mospace-before.json`。

已在真实生产 MoSpace client 的授权页确认：默认“手机登录”、手机号／验证码、“登录 / 注册”、自动注册提示、协议和原 Logo 均显示；已有会话的“使用以下账号继续”保留。该检查未提交模拟 state 的授权请求。真实 MoSpace 回跳由用户从 MoSpace 重新发起登录验收。
