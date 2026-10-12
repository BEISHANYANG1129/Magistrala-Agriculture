# 本机 Bootstrap UI 路径兼容修复

2026-10-11 经用户授权部署。当前官方 UI 的 SDK 使用 `clients/configs`、`clients/bootstrap`，本机 Bootstrap 后端已经迁移到 `devices/configs`、`devices/bootstrap`。旧路径返回纯文本 404，UI 再按 JSON 解析导致页面报错。

本派生镜像只修正四个服务端 chunk 中的 Bootstrap 路径，不修改认证、权限或数据。基础镜像为本机 `magistrala-ui-remembered:20261011`，继承已有长期登录修复；会话继续使用原有持久化数据卷。

构建及部署（从仓库根目录执行）：

```powershell
docker build --pull=false -t magistrala-ui-bootstrap-compat:20261011 docker/ui-bootstrap-compat
# docker/.env 中 MG_UI_IMAGE=magistrala-ui-bootstrap-compat:20261011
wsl -d Ubuntu-20.04 --cd /mnt/d/personal/magistrala/magistrala/docker docker compose -p absmach_magistrala --env-file .env --env-file .env.tokens up -d --no-deps ui
```

补丁先确认预期路径存在，并通过 JavaScript 语法检查，然后写入文件。它针对本机固定 UI 构建；升级时需重新核对 chunk 与接口，不能盲目应用。

已验证：原路径 404；新路径无凭据时返回鉴权 JSON。部署后复用已有浏览器会话，Configurations、Profiles 列表及 Create Profile 表单正常打开，无原 JSON 解析错误。没有创建引导记录或执行设备引导。

回退：把 `.env` 的 `MG_UI_IMAGE` 改回 `magistrala-ui-remembered:20261011`，再运行同一条仅更新 `ui` 的 Compose 命令。保留会话卷和数据库。
