FROM docker.m.daocloud.io/library/golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN /usr/local/go/bin/go env -w GOPROXY=https://goproxy.cn,direct && /usr/local/go/bin/go mod download
COPY . .
RUN CGO_ENABLED=0 /usr/local/go/bin/go build -o /out/orderapp ./

FROM docker.m.daocloud.io/library/alpine:3.20
WORKDIR /app

# For git sync on acceptance: git + ssh client
RUN apk add --no-cache git openssh-client ca-certificates

COPY --from=build /out/orderapp /app/orderapp
COPY templates /app/templates
COPY docs /app/docs
EXPOSE 8080
ENV LISTEN=:8080
CMD ["/app/orderapp"]
