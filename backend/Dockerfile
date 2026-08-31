FROM golang:1.22-alpine AS build

ARG GOPROXY=https://mirrors.aliyun.com/goproxy/
ENV GOPROXY=${GOPROXY},https://proxy.golang.org,direct
WORKDIR /src/backend
COPY backend/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -mod=vendor -trimpath -ldflags="-s -w" -o /out/hep-api ./cmd/server

FROM alpine:3.20
RUN addgroup -S hep && adduser -S -G hep hep
WORKDIR /app
COPY --from=build /out/hep-api /app/hep-api
COPY backend/migrations /app/migrations
USER hep
EXPOSE 8080
ENTRYPOINT ["/app/hep-api"]
