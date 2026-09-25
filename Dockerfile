FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS build

ARG TARGETOS
ARG TARGETARCH

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
COPY ui ./ui
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-s -w" -o /out/pal ./cmd/web

FROM alpine:3.22

RUN apk add --no-cache ca-certificates \
    && adduser -D -H -u 10001 pal \
    && mkdir /data \
    && chown pal:pal /data

WORKDIR /app
COPY --from=build /out/pal ./pal
COPY --from=build /src/ui ./ui
USER pal

ENV DB_PATH=/data/library.db \
    PORT=8080 \
    OPENLIBRARY_USER_AGENT="pal/1.0 (personal library)" \
    GOOGLE_BOOKS_API_KEY=""

EXPOSE 8080
ENTRYPOINT ["/app/pal"]