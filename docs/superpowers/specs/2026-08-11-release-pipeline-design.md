# Release Pipeline Design（GitHub Actions + 双仓库 Docker 发布）

日期：2026-08-11
状态：待审阅

## 1. 目标

推送 `v*` tag 时自动：测试 → 构建（GUI + Linux 二进制）→ 创建 GitHub Release → 发布多架构 Docker 镜像到 GHCR 与 Docker Hub。

## 2. 触发与镜像版本约定

- 触发：`push: tags: ['v*']`
- Docker tag：`0.1.2`（去 v 的 semver）+ `latest`（每次 tag 发布都更新）
- 发布对象：
  - `ghcr.io/riverisagame/searchmiddleware:{0.1.2, latest}`
  - `docker.io/dockerdoo/searchmiddleware:{0.1.2, latest}`

## 3. 工作流（.github/workflows/release.yml）

### Job 1：test-build（构建产物上传 artifact）
1. checkout
2. setup-node 20 → `npm ci && npm run build`（web-ui → dist）
3. 拷贝 `web-ui/dist` → `internal/web/dist`（go:embed 使用）
4. setup-go 1.26.5
5. `go test ./...`
6. 交叉编译两个 Linux 二进制（CGO_ENABLED=0）：
   - `GOOS=linux GOARCH=amd64` → `searchmiddleware-linux-amd64`
   - `GOOS=linux GOARCH=arm64` → `searchmiddleware-linux-arm64`
7. `actions/upload-artifact` 上传两个二进制

### Job 2：release（依赖 Job 1）
- `softprops/action-gh-release`（GITHUB_TOKEN 自动认证）
- 附件：两个 Linux 二进制

### Job 3：docker（依赖 Job 1）
1. checkout + 下载二进制 artifact 到仓库根目录
2. `docker/setup-qemu-action` + `docker/setup-buildx-action`（multi-arch）
3. 登录：GHCR（GITHUB_TOKEN）+ Docker Hub（`DOCKERHUB_USERNAME`/`DOCKERHUB_TOKEN`）
4. `docker/build-push-action`：
   - platforms: `linux/amd64,linux/arm64`
   - tags（一次构建推双仓库 4 个 tag）：
     - `ghcr.io/riverisagame/searchmiddleware:0.1.2`
     - `ghcr.io/riverisagame/searchmiddleware:latest`
     - `docker.io/dockerdoo/searchmiddleware:0.1.2`
     - `docker.io/dockerdoo/searchmiddleware:latest`
   - 不重复编译：镜像内直接 COPY CI 产物

## 4. Dockerfile（单阶段，纯复用 CI 产物）

```dockerfile
FROM alpine:3.20
ARG TARGETARCH
COPY searchmiddleware-linux-${TARGETARCH} /usr/local/bin/searchmiddleware
ENTRYPOINT ["/usr/local/bin/searchmiddleware"]
```

要点：二进制在 CI 编译并 embed GUI，镜像仅做拷贝；`TARGETARCH` 由 buildx 自动注入选择对应产物。

## 5. .dockerignore

排除 `.git/`、`data/`、`web-ui/node_modules/`、`web-ui/dist/`、`internal/web/dist/`、`*.log`、`cmd/` 诊断程序等，避免上下文膨胀与密钥泄漏。

## 6. 所需 Secrets（仓库级）

| Secret | 值 | 说明 |
|--------|-----|------|
| `DOCKERHUB_USERNAME` | `dockerdoo` | Docker Hub 登录用户 |
| `DOCKERHUB_TOKEN` | 用户生成 | Docker Hub Access Token |
| （GHCR） | 内置 `GITHUB_TOKEN` | 无需配置 |

## 7. 不做的事（YAGNI）

- 不发布 Windows/macOS 二进制（已确认只发 Linux）
- 不自动更新 CHANGELOG
- 不做 PR 触发构建
- 不做 SBOM/signing
- 不做 Docker Hub 描述同步
