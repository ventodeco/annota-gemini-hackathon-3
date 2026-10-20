FROM oven/bun:1.4.0-alpine AS bun

FROM node:22-alpine AS builder

# Keep Node available for build tools, using Bun from the pinned official image.
COPY --from=bun /usr/local/bin/bun /usr/local/bin/bun

ARG VITE_API_BASE_URL
ARG VITE_APP_URL

ENV VITE_API_BASE_URL=$VITE_API_BASE_URL
ENV VITE_APP_URL=$VITE_APP_URL

WORKDIR /app

COPY package.json bun.lock ./
COPY web/package.json ./web/package.json
RUN bun install --frozen-lockfile

COPY web/ ./web/

WORKDIR /app/web
RUN bun run build

FROM nginx:alpine

COPY --from=builder /app/web/dist /usr/share/nginx/html
COPY web/nginx.conf /etc/nginx/conf.d/default.conf

EXPOSE 80

CMD ["nginx", "-g", "daemon off;"]
