FROM node:24 AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web ./
RUN npm run build

FROM golang:1.27 AS api
ARG VERSION=dev
WORKDIR /src/api
COPY api/go.mod api/go.sum ./
RUN go mod download
COPY api ./
RUN CGO_ENABLED=0 go build -ldflags "-X main.version=${VERSION}" -o /out/api ./cmd/api

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=api /out/api /app/api
COPY --from=web /src/web/dist /app/web
ENV WEB_DIR=/app/web
EXPOSE 8080
ENTRYPOINT ["/app/api"]
