// Package web 内嵌管理后台 SPA 的构建产物（web/dist）。
//
// M1 仅内嵌一个静态占位 index.html；M3 接入真正的 React 构建后，dist 目录由前端产物覆盖。
package web

import "embed"

// DistFS 内嵌 dist 目录的全部静态资源。
//
//go:embed dist
var DistFS embed.FS
