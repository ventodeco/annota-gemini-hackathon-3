FROM node:22-alpine AS builder

WORKDIR /app/frontend

RUN npm install -g bun

COPY web/package.json web/bun.lock* ./
RUN bun install --frozen-lockfile

COPY web/src src
COPY web/public public
COPY web/index.html .
COPY web/vite.config.ts .
COPY web/tsconfig.json .
COPY web/tsconfig.app.json .
COPY web/tsconfig.node.json .
COPY web/tsconfig.vitest.json .
COPY web/components.json .
COPY web/eslint.config.js .

RUN bun run build

FROM nginx:alpine

COPY --from=builder /app/frontend/dist /usr/share/nginx/html

EXPOSE 80

CMD ["nginx", "-g", "daemon off;"]
