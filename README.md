# Proxy Hub

自托管的 OpenAI 兼容多上游渠道代理网关。

## 当前状态

项目仍处于早期实现阶段。当前基础能力包括：

- Go 模块与 HTTP 启动骨架
- `GET /healthz`
- YAML 配置结构、校验、默认值规范化、原子保存与文件监听
- 示例配置
- 最小 React/Vite 前端骨架

## 后端

```powershell
go run ./cmd/proxy-hub --config ./config.example.yaml --no-browser
```

健康检查：

```powershell
curl http://localhost:8787/healthz
```

## 前端

```powershell
cd web
pnpm install
pnpm dev
```

## 验证

```powershell
go test ./...
go build ./...
cd web
pnpm build
```
