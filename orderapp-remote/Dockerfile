FROM docker.m.daocloud.io/library/golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN /usr/local/go/bin/go env -w GOPROXY=https://goproxy.cn,direct && /usr/local/go/bin/go mod download
COPY . .
RUN /usr/local/go/bin/go test ./...
RUN CGO_ENABLED=0 /usr/local/go/bin/go build -o /out/orderapp ./

FROM docker.m.daocloud.io/library/alpine:3.20
WORKDIR /app

COPY --from=build /out/orderapp /app/orderapp
COPY templates /app/templates
COPY docs /app/docs
COPY assets /app/assets
# Vue shell static files
COPY frontend-vue-shell/dist /app/frontend-vue-shell/dist
EXPOSE 8080
ENV LISTEN=:8080
CMD ["/app/orderapp"]
