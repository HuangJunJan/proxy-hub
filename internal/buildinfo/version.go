// Package buildinfo 持有构建期注入的版本信息。
package buildinfo

// Version 是二进制版本号。默认 "dev"，发布构建时通过
// `-ldflags="-X github.com/huangjunjan/proxy-hub/internal/buildinfo.Version=<版本>"` 注入。
var Version = "dev"
