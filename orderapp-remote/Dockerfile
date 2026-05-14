FROM docker.m.daocloud.io/library/golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN /usr/local/go/bin/go env -w GOPROXY=https://goproxy.cn,direct && /usr/local/go/bin/go mod download
COPY . .
RUN set -e; \
    mkdir -p /docs /miniapp; \
    if [ -d docs ]; then cp -R docs/. /docs/; fi; \
    if [ -d miniapp ]; then cp -R miniapp/. /miniapp/; fi; \
    cp docs/OP_MANUAL_*.md / 2>/dev/null || true; \
    cp docs/REQUIREMENTS.md docs/ACCEPTANCE_TESTS.md docs/HOW_TO_WORKFLOW.md docs/OPERATION_MANUALS.md / 2>/dev/null || true; \
    /usr/local/go/bin/go test ./...
RUN CGO_ENABLED=0 /usr/local/go/bin/go build -o /out/orderapp ./

FROM docker.m.daocloud.io/library/alpine:3.20
WORKDIR /app
RUN sed -i 's|https://dl-cdn.alpinelinux.org/alpine|https://mirrors.aliyun.com/alpine|g' /etc/apk/repositories \
    && apk add --no-cache font-noto-cjk libreoffice ttf-dejavu

COPY --from=build /out/orderapp /app/orderapp
COPY templates /app/templates
COPY docs /app/docs
COPY assets /app/assets
# Vue shell static files
COPY frontend-vue-shell/dist /app/frontend-vue-shell/dist
EXPOSE 8080
ENV LISTEN=:8080
ENV DOCX_CONVERTER_CMD=soffice
CMD ["/app/orderapp"]
