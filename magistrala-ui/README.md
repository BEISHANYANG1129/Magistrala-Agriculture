# 农业管理前端

Vue 3 / Vite 应用。完整平台部署见 [根目录 README](../README.md)。

开发：`npm ci`，然后 `npm run dev`。构建与类型检查：`npm run build`。工作流测试：`npx vitest run`。

生产 Docker 部署同时提供静态构建产物、同源 API 转发、服务端登录会话和区域分析服务。主机运行构建产物时使用 `node ../scripts/serve.mjs`，不要只部署 dist 静态目录。

登录会话细节见 [server/README.md](server/README.md)，功能与验收边界见 [AGRI-UI-UPGRADE.md](../docs/AGRI-UI-UPGRADE.md)。
