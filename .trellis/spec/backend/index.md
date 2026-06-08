# Backend Development Guidelines

> Best practices for backend development in this project.

---

## Overview

This directory contains guidelines for backend development. Fill in each file with your project's specific conventions.

---

## Guidelines Index

| Guide | Description | Status |
|-------|-------------|--------|
| [Directory Structure](./directory-structure.md) | Module organization and file layout | ✅ 已填（M1） |
| [Database Guidelines](./database-guidelines.md) | ORM patterns, queries, migrations；sqlc 约定（M2） | ✅ 已填（M1/M2） |
| [Relay / Channel](./relay-channel.md) | 渠道/路由/适配/冷却/UsageEvent 与安全不变量 | ✅ 已填（M2） |
| [Error Handling](./error-handling.md) | Error types, handling strategies | ✅ 已填（M1） |
| [Quality Guidelines](./quality-guidelines.md) | Code standards, forbidden patterns；跨平台文件 I/O（M2） | ✅ 已填（M1/M2） |
| [Logging Guidelines](./logging-guidelines.md) | Structured logging, log levels | ✅ 已填（M1） |

---

## How to Fill These Guidelines

For each guideline file:

1. Document your project's **actual conventions** (not ideals)
2. Include **code examples** from your codebase
3. List **forbidden patterns** and why
4. Add **common mistakes** your team has made

The goal is to help AI assistants and new team members understand how YOUR project works.

---

**Language**: 本项目规范文档用**中文**撰写（与 `AGENTS.md`、`design.md` 一致）；代码标识符、表名/列名、路径、API 路径、技术名保留英文。
