FROM oven/bun:1-alpine AS build

WORKDIR /src

COPY package.json bun.lock ./
RUN bun install --frozen-lockfile

COPY . .
RUN bun run build

FROM oven/bun:1-alpine

WORKDIR /app

COPY --from=build /src/.output /app/.output

ENV NODE_ENV=production
ENV HOST=0.0.0.0
ENV PORT=3000

EXPOSE 3000

CMD ["bun", "/app/.output/server/index.mjs"]
