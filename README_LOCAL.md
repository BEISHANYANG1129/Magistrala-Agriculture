# 本地运行

请先阅读仓库根目录 [README.md](README.md) 和 [部署说明](docs/DEPLOYMENT.md)。

初始化：`node scripts/deploy.mjs init`。完整 Docker 部署：`node scripts/deploy.mjs up`。

本地源码开发需要 PATH 中的 Node.js 22.12+ 和 Go 1.24.4+。安装农业前端依赖后，在仓库根目录分别启动：

```bash
node scripts/run-local.mjs ui
node scripts/run-local.mjs agent-ui
```

Windows 兼容入口：`powershell -ExecutionPolicy Bypass -File .\run-local.ps1 ui`。

前端地址 `http://127.0.0.1:5173`。不要同时运行占用相同端口的 Docker 农业容器和本机开发服务。可选的 image、control、llm、integration、executor 参数见主 README；第三方集成和真实执行器需要自己的凭据及 API 适配。
