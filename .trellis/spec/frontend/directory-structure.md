# 前端目录结构与工程约定（Directory Structure）

> proxy-hub M3 确立的前端约定（首个前端里程碑）。语言：与项目一致用中文。

---

## 技术栈

React 18 + TypeScript + Vite 5 + Tailwind CSS 3 + TanStack Query 5 + recharts。无 UI 框架重依赖（用 Tailwind 手写组件，保持产物精简）。

## 目录布局

```
web/
  package.json            # 依赖与脚本（build = tsc --noEmit && vite build）
  vite.config.ts          # dev 端口 8888 + proxy 到后端 7777；build.outDir=dist
  tsconfig.json           # strict + noUnusedLocals/Parameters；include src + vite.config.ts
  tailwind.config.js / postcss.config.js
  index.html              # Vite 入口模板（<div id="root">）
  src/
    main.tsx              # 挂载 + QueryClientProvider
    index.css             # tailwind 指令
    api.ts                # fetch 客户端 + 类型 + admin key（localStorage）
    App.tsx               # 布局 + admin key 闸门 + 标签页 + 区间选择
    pages.tsx             # 概览/趋势/分组/请求日志/健康/定价 各页 + 共享小组件
  dist/                   # 构建产物（被 web/embed.go 的 go:embed 收录；提交入库）
```

## 端口与部署（核心约定）

- **开发**：`npm run dev` 起 Vite 于 **8888**；`vite.config.ts` 的 `server.proxy` 把 `/v1`·`/admin`·`/v0`·`/healthz` 代理到 **`http://127.0.0.1:7777`**。前后端分离调试。
- **生产无独立前端端口**：`npm run build` 产出 `web/dist`，由后端经 `web/embed.go` 的 `//go:embed dist` 单端口（7777）提供；M1 的 `registerStatic` 已实现 SPA history 回退。
- **`web/dist` 提交入库**（`.gitignore` 有 `!web/dist/` 例外）；`node_modules` 忽略。改前端后须 `npm run build` 再提交，保持 dist 与源码一致。

## 鉴权

admin key 由用户输入一次、存 `localStorage`（`proxy-hub-admin-key`）；`api.ts` 对每个 `/admin/*` 请求注入 `Authorization: Bearer <key>`。401 即视为 key 失效（前端可清除重输）。**绝不**把 key 写进代码或日志。

## 数据获取

统一经 `api.ts` 的类型化 fetcher + TanStack Query（`useQuery`/`useMutation`）。查询键含过滤/区间参数以便缓存与失效；改价等写操作成功后 `invalidateQueries`。

## 质量门

`npm run build`（= `tsc --noEmit` 严格类型检查 + `vite build`）必须干净；与后端的 `go build`/`go test` 一道作为 M3 评审门。recharts 致产物 >500KB（仅告警，后续可 code-split）。
