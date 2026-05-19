FROM node:22-alpine AS builder
WORKDIR /app

COPY web/package.json web/pnpm-lock.yaml web/pnpm-workspace.yaml ./
COPY web/ .
COPY api/openapi.yaml /api/openapi.yaml
RUN corepack enable && pnpm install --frozen-lockfile && pnpm build

FROM nginxinc/nginx-unprivileged:1.27-alpine
COPY --from=builder /app/dist /usr/share/nginx/html
EXPOSE 8080
