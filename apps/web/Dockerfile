# syntax=docker/dockerfile:1

FROM node:20-alpine AS deps
WORKDIR /app
COPY package.json pnpm-workspace.yaml turbo.json ./
COPY apps/web/package.json ./apps/web/
COPY packages/schemas/package.json ./packages/schemas/
RUN corepack enable && pnpm fetch

FROM node:20-alpine AS builder
WORKDIR /app
COPY --from=deps /pnpm-lock.yaml ./pnpm-lock.yaml || true
COPY --from=deps /app/node_modules ./node_modules || true
COPY . .
RUN corepack enable && pnpm install --frozen-lockfile
RUN pnpm --filter @streamlation/web build

FROM node:20-alpine
WORKDIR /app
ENV NODE_ENV=production
COPY --from=builder /app/apps/web/.next ./.next
COPY --from=builder /app/apps/web/package.json ./package.json
COPY --from=builder /app/node_modules ./node_modules
EXPOSE 3000
CMD ["pnpm", "start"]
