FROM node:22-alpine AS builder

WORKDIR /app/frontend

RUN npm install -g bun

COPY web/package.json web/bun.lock* ./
RUN bun install

COPY web/ .

RUN bun run build

FROM nginx:alpine

COPY --from=builder /app/frontend/dist /usr/share/nginx/html

EXPOSE 80

CMD ["nginx", "-g", "daemon off;"]
