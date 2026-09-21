# SMB 补充 MCP：第一阶段

目标为 QuTS hero 6.0.2；这不是已通过真机兼容验收的声明。
部署前必须确认实际固件完整版本及 build，并核对安装目标。

2026-09-21 经现有 QNAP MCP 的 get_system_info 只读回查：
目标 NAS94E4A0 / TS-673A，地址 192.168.1.50，
固件名称 QuTS hero，版本 h6.0.2，build 20260819，patch 0。
这是设备身份与固件证据，不是原生权限接口兼容性证明。
随后经用户授权只读 SSH 到 192.168.1.50:2222，确认 eth1 地址、
主机名与上述目标一致；QCLI、os-release 和 default_config 均报告
6.0.2 / 20260819。运行配置 uLinux.conf 仍记录 6.0.1 / 20260709，
不得单凭该旧字段选择适配器。未安装程序、修改权限或创建共享。

本 fork 独立于 FA 业务仓库，沿用 Go Agent 和 Node MCP Bridge。
第一阶段只允许健康检查、共享清单、SMB 状态、目录列表、
文件元数据、读取及 SHA-256 校验。它们不证明真实 SMB 账号读写成功。

## 配置

Agent 使用 `configs/smb-supplement-readonly.json`：

- 替换 token hash 和实际存在的验收目录；示例不会创建共享。
- 保持 observe、supplement_read_only=true、allowed_roots 为窄范围。
- 不通过 full_trust 初始化流程覆盖该配置。
- loopback 监听需经已有可信通道访问，不直接暴露到公网。
- 共享清单和 SMB 状态是 NAS 级元数据，不受文件 allowed_roots 过滤。

Bridge 设置：

```text
QACS_SUPPLEMENT_READ_ONLY=true
QACS_TOOLSETS=core,files,qnap
QACS_BASE_URL=<可信 Agent 地址>
QACS_TOKEN=<运行时注入，不提交仓库>
```

两端开关必须同时配置。只隐藏 MCP 工具不构成服务端安全边界；
Agent 对非白名单 HTTP 方法及路径返回 403，并沿用审计。
配置加载会拒绝 full_trust、根路径 /、任意命令、shell 和自定义 adapter，
并要求审计与敏感信息脱敏开启。HTTP 白名单不依赖工具列表隐藏或审批。

## 明确未实现

- hero 显式 ACE 删除及继承来源识别。
- QNAP 网络回收站删除和单文件恢复。
- 安全的快照单文件恢复。
- 实际 SMB 客户端上传、下载及恢复内容一致性演练。

不将 getfacl/POSIX 权限作为 hero Windows ACL 的完整视图。
禁止用 setfacl、任意 shell、os.Remove 或 zfs rollback 冒充上述能力。
这些写接口在补充模式下不可调用。

后续只能在独立测试共享确认接口、固件适用性、前后状态及超时行为后，
逐个添加窄范围能力。不得依靠省略 ACL 条目执行未经验证的删除。

## 权限配置由 QNAP 系统负责

补充 MCP 不实现独立 ACL 引擎，不生成 POSIX ACL、richacl 或 Windows ACE，
不修改 smb.conf，不直接调用 setfacl，也不建立另一套权限账本。

优先使用 QNAP 官方 MCP 已验证的系统配置操作；其能力不足时，
先通过 QuTS hero 6.0.2 原生管理页面在隔离测试共享中确认操作行为。
原生页面可操作，不等于它背后的私有 API 已成为稳定公开接口。
只有取得明确接口契约并验证具体固件 build 后，才考虑封装同一系统操作。
无法确认契约时保持只读，不能猜测 CGI 参数或用 raw shell 回退。

底层由系统处理权限继承和持久化；适配层仅负责限定目标、
操作审计、避免同一目标并发写入、查询系统任务状态和结果回读。
超时不等于系统任务取消，不自动重发；系统原生操作也不能保证避开
已有的内核锁等待，转换或阻塞期间仍须停止权限变更。

本阶段尚未接通原生权限写入，也没有为未知接口创建自动执行框架。

## h6.0.2 原生界面契约取证

2026-09-21 只读检查本机安装文件：
`/home/httpd/cgi-bin/apps/systemPreferences/functions/sharedFolders.js`。
以下是该 build 前端生成的请求，不是公开 API 承诺，也没有执行写请求。

基本权限视图的 `saveBasic` 向
`/cgi-bin/priv/AccessControl.cgi` 发起 POST：

```text
func=set_ntfs_basic
target=<当前界面的共享/子目录路径>
action_count=1
action1=delete
len1=<待删除账号数量>
ntfs1-1=<读取结果中的 principal>,<读取结果中的 name>
```

界面还带有随机 count 参数。principal 和 name 必须来自系统查询，
不得从 FA 账号类型推断。这个操作按账号删除基本视图权限，
不能宣称它只删除一个细粒度 deny ACE。

对应读接口为 `func=get_ntfs_basic`，参数包含 target、getdataex=1、
filter、type、lower、upper、refresh。界面声明 XML 列表为 `list/node`，
总数为 `list/count`，但本机真实响应未提供 count，见下方实测限制。
记录包含 principal、name、ace_no、ace_ro、ace_rw、ace_win。
界面将枚举 3 显示为禁用且选中，并在 replace 时视为继承权限；
真实归属仍需结合系统继承状态及详细视图确认。

Windows 基本和特殊视图分别调用 set_ntfs_win_basic、
set_ntfs_win_special，删除项编码不同，不能混用：

- Windows 基本视图为每个账号提交两条 principal,name，len 为账号数的两倍。
- 特殊视图提交 principal,name,type,ace.slice(1,14),apply_onto,container_only。
- 基本视图仅提交 principal,name。

继承操作是独立的 actionN=inheritance / modeN：
enable 对应开启继承；removeall 对应关闭继承并移除继承项；
explicit 对应关闭继承并转换为显式项。removeall 不是通用清空 ACL。
本任务不应调用继承转换或递归 replace。

界面在 richacl_converting 非零时禁用编辑，并在提交前检查共享转换列表。
提交完成后延迟重新加载列表，还检查 over_entries。
适配器必须另外确认响应业务结果及删除后回读，不能只凭 HTTP 200
或回调成功声称删除完成；写超时必须标记结果未知，禁止自动重试。

QCLI 二进制帮助提供 `qcli_sharedfolder -M` /
`--removeusergrouppermission`，接受 sharename、user、group、computer。
同一二进制含旧 privWizard.cgi / share_access_control 字符串，
尚无证据证明 -M 使用上述 ACL 2.0 删除接口，因此没有执行该命令。

下一门禁：取得只读查询的真实响应和继承来源，核对认证及业务错误契约，
然后在另行授权的隔离测试共享执行一次原生删除并回读。
这些源码证据解释了“省略条目不等于删除”的适配缺口，
但不证明此前内核等待的具体原因，也不代表生产旧权限已清理。

## 原生只读实测与当前阻塞

2026-09-21 使用运行时凭据在内存中认证，直接调用上述系统读接口：

| 目标 | 返回项数 | 实测结果 |
| --- | --- | --- |
| xigu-fa | 26 | 人员及设备账号 RW，保留 guest 拒绝及管理权限 |
| xigu-fa/_device-inbox | 25 | 5 个人员账号同时 ace_rw=3、ace_no=1；17 个设备账号 ace_ro=1 |
| xigu-fa/_device-inbox/xgic | 未取得 | 15 秒读取超时，停止后续流程 |

前两处返回 richacl_converting=0。它不能证明 xgic 目录没有阻塞。
父目录 enable_inherited=0、inherited_mode=1，不能将这两个字段合并
成一个布尔值，也不能把同时存在的 RW 与拒绝压缩成单一 RW 标签。

真实 XML 在 func/ownContent/ntfs/list 下返回 node，但没有 count。
将 lower/upper 改为 1/2 后，根和父目录仍返回相同的 26/25 项。
这证明这些请求未按预期分页，不足以证明服务端没有其他截断规则。
不应将返回长度伪装成服务器声明的总数。

后续只读 SSH 状态检查返回认证拒绝，未重试登录。
没有获得当前内核等待证据，不能据此断言内核死锁或账号已被封禁。
未创建隔离目录、未调用任何权限写接口、未清理生产权限。

新增 `agent/internal/qnap/nativeacl` 原生只读模块：

- 使用 Go 标准 XML 解析器，保留混合枚举及两个独立继承字段。
- 缺少 count 时返回 Complete=false；未知枚举或缺少状态时报错。
- 精确目标白名单、15 秒总期限、限制响应体大小。
- 会话放在 POST 表单内，不跟随重定向、不自动重试、不输出底层认证信息。
- 没有删除接口，尚未注册为 MCP 工具或部署；不改变既有只读白名单。

下一步必须先恢复稳定的目标目录查询及完整性确认，再开展隔离目录删除。
本次新增测试仅覆盖原生读取模块；不等同于线上清理、SMB 或恢复验收。

## 后续复查：已取得详细来源，仍未清理

同日再次经用户要求继续复查：

- SSH 恢复；主机、eth1 地址和 default_config 再次确认是目标 6.0.2 / 20260819。
- xgic 基本视图成功返回 25 个账号；不是服务器声明的 ACE 总数。
- get_ntfs_win_special 返回逐条 ACE、inherited 和 ancestor。
  xgic 的 5 个人员账号同时有本地显式拒绝和来自 `_device-inbox` 的继承拒绝。
- 父目录详细视图返回 33 条 ACE，5 条人员拒绝均 inherited=0、ancestor=none。
  因此必须先处理父级来源，再回读子级；不能只在 xgic 删除继承项。
- xgic 详细视图中，NAS_FA 和 administrators 的继承拒绝标记来源为
  `Parent object`；父目录详细视图却只返回这些账号的允许项。
  此来源尚未解析，不推断实际有效权限，也不将其纳入自动删除。
- richacl PID 11615 两次采样持续为 R，内核 CPU ticks 从 1174085
  增至 1174296，采样间隔约 2.13 秒。fs_listener 主线程为 D，
  查询期间还采样到 AccessControl.cgi 为 D。随后查询成功返回，
  因此这些采样证明发生等待，但不证明永久死锁。
- 管理员只读提权读取进程栈未返回栈内容；过滤的内核日志无匹配记录。
  没有获得足够证据解释具体内核等待原因。未发送信号或重启服务。

原生模块新增 ReadDetailed / ParseDetailed，保留同账号多条显式、继承
以及重复 ACE 和原始 ancestor，不将 `Parent object` 猜成真实目录。
定向测试覆盖来源保留、无总数和无效详细条目。

增加本地只读命令 `agent/cmd/qnap-acl-inspect`：

```sh
go build -o /tmp/qnap-acl-inspect ./cmd/qnap-acl-inspect
# QNAP_NATIVE_SID 必须由已有认证流程在内存中注入，不写入文件或命令参数。
/tmp/qnap-acl-inspect --base-url <NAS地址> --target xigu-fa/_device-inbox/xgic
```

输出系统详细权限 JSON。退出码 1 表示读取失败，2 表示参数不完整，
3 表示成功取得只读证据但完整性未确认或仍在转换，不能推进写入。
命令不创建会话，不提供删除操作，不部署到 NAS，不改变 MCP 白名单。
仍未执行隔离目录删除、生产清理或 SMB 恢复验收。

## 后续实施：隔离删除通过，人员拒绝已定点清理

以下同日实施结果覆盖上文“未执行”的历史状态，不表示完整 SMB 验收：

1. refresh=1 逐层查询后，xgic 仍返回 58 条详细 ACE，管理员来源
   `Parent object` 未消失。25 是基本视图账号数，不是实际 ACE 数。
2. get_effective 按本机页面请求契约调用，仅返回空 QDocRoot；
   不能当作允许、拒绝或 SMB 读写证明。
3. 经 File Station 原生 createdir 创建空目录
   `xigu-fa/_acl-acceptance-20260921-c64ecbb824`。初始 26 条继承项，
   后续出现 administrators/Everyone 两条显式允许；连续两次确认
   28 条基线稳定后保留它们，不推断是谁添加。
4. 仅在空目录为一个既有人员账号添加显式 RO。15 秒请求超时，
   回读却确认条目已落盘，成为 29 条。未重试添加。
5. 原生 set_ntfs_win_special 精确删除该测试 ACE，返回 result=1；
   回读恢复 28 条，其余条目逐项一致。测试空目录保留，未删除文件。
6. `_device-inbox` 先删除 1 条人员拒绝并回读通过，再清理剩余
   4 条人员拒绝及 17 条设备 RO。后一次写超时，但回读确认已删除。
7. 设备账号未出现预期继承 RW，因此按清理前快照恢复全部 17 条
   原始设备 RO，逐项回读一致。管理员、guest 等非目标项未变。
8. 随后仅删除 xgic 自身 5 条人员显式拒绝。首次回读转换状态未通过，
   没有重发写请求。后续只读核验：5 个账号显式项已移除、无剩余
   deny、继承 RW 均可见，其余显式项与快照一致。

最终已确认的净变化：父目录与 xgic 合计删除 10 条人员显式拒绝。
设备 RO 保留原状，不将其描述为设备权限已修复。
没有清理其他设备子目录、人员目录或管理员未知来源项。
没有进行真实 SMB 上传下载、回收站恢复或版本恢复验收。

本机前后快照和操作回执（不含会话或密码）：

```text
/Users/zhd/Documents/Codex/qnap-acl-acceptance-20260921/
  parent-canary-20260921T055657Z.json
  parent-cleanup-20260921T055913Z.json
  xgic-personnel-20260921T060621Z.json
```

parent-cleanup 的删除后验收为 partial_or_unconfirmed，
rollback_phase=verified 表示设备 RO 已恢复，不是删除验收通过。
xgic-personnel 的最终状态为 verified_after_reconcile。
超时或紧接写入后的转换状态变化必须进入只读结果核对，不能直接重试。
本轮没有部署 fork 或将权限写操作暴露为 MCP 工具。

## 后续实施：原生继承刷新后设备冗余 RO 已清理

以下同日结果覆盖上节“设备 RO 保留原状”的最终状态：

1. 只读链路确认：共享根有 17 条设备 RW；`_device-inbox` 有
   17 条显式 RO，却没有设备继承项；xgic 继承到的是父级旧 RO。
   被抽查设备自身目录有本设备显式 RW，因此父级 RO 不等于上传必然失败。
2. 使用 smbprotocol 以目标设备账号和运行时初始密码执行一次真实
   SMB 登录，返回 LogonFailure。没有重置密码、重试猜测或执行文件写入。
   这只证明该凭据未通过认证，不证明 ACL 拒绝。
3. 在既有空验收目录调用系统原生
   `set_ntfs_win_special / action1=inheritance / mode1=enable`，
   返回成功，28 条 ACE 前后逐项一致。这不是格式转换、removeall
   或递归 replace。
4. 对 `_device-inbox` 做同样操作后，17 个设备继承 RW 全部出现，
   全部显式项保持原状。enable_inherited/inherited_mode 两个字段仍是
   0/1，因此不能仅凭它们断言继承完整；必须实际检查账号继承项。
5. 在确认每个设备都有继承 RW 后，删除父目录 17 条冗余设备显式 RO；
   逐项回读通过，非目标显式项未变。
6. 对 xgic 运行同样的原生继承操作；首次立即回读状态未通过，
   后续仅查询确认 17 个设备继承 RW 可见，显式项未变，
   先前来源为 Parent object 的两项不再返回。没有重发写请求。
7. 删除 xgic 的 17 条设备显式 RO，回读确认全部移除且设备继承 RW
   仍在，非目标显式项未变。

本轮净变化：两层删除 34 条设备显式 RO，改用系统恢复的继承 RW。
结合上轮，已清理这两层的 10 条人员拒绝及 34 条设备 RO。
这不表示所有下级设备目录的历史 ACL 都已清理。
没有直接修改管理员、guest 或共享根权限，也没有触碰文件内容。

新增回执位于同一验收目录：

```text
isolated-enable-inheritance.json
parent-enable-inheritance-20260921T061600Z.json
parent-devices-20260921T061756Z.json
xgic-enable-inheritance-20260921T062002Z.json
xgic-devices-20260921T062223Z.json
```

parent-devices 和 xgic-devices 的最终 phase 均为 verified；
xgic-enable-inheritance 为 verified_after_reconcile。
原生操作证实可以补齐本机缺失的设备继承项，但不证明历史内核等待
已修复，也不保证其他固件或目录具有相同行为。

剩余验收：取得设备账号当前有效凭据；验证实际 SMB 列目录、上传、
下载及恢复；核查下级设备与阶段目录。fork 仍未部署，写操作未注册为
MCP 工具。不得把原生权限回读通过描述为 SMB 业务全链路通过。
