FROM node:22-alpine AS builder

ARG VITE_API_BASE_URL
ARG VITE_APP_URL

ENV VITE_API_BASE_URL=$VITE_API_BASE_URL
ENV VITE_APP_URL=$VITE_APP_URL

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
COPY web/nginx.conf /etc/nginx/conf.d/default.conf

EXPOSE 80

CMD ["nginx", "-g", "daemon off;"]
