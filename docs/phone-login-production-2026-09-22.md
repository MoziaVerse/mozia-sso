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
- 真实生产短信、新号码注册及真实 Matrix 授权回跳仍需用户操作验收，不能用模拟短信结果替代。

## 运行与回退

使用原 `docker-compose.mozia.yml` 加 `docker-compose.phone-login.yml` 启动 SSO。原文件会话复制到 `session-data` 并挂载 `/tmp`，后续重建须继续保留该卷。

生产受限备份位于 `~/app/mozia-sso-backups/20260922-phone-login`，包括发布前数据库、配置、旧镜像标识及 `rollback.sh`。回退脚本恢复旧配置和旧镜像，保留会话卷及已创建用户，不覆盖用户数据库。新增列兼容旧版。该脚本已准备，本次未为了演练再中断生产。
