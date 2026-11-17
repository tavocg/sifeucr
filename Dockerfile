# syntax=docker/dockerfile:1@sha256:4edf897a3ffa55b89f906fc8cc78afdb3f1834cc9c7083565e611a8a7d5fe99e
FROM golang:1.26.11-alpine@sha256:ae52b5bc6ed045a9ed26ab03dc19327a802a9cc9aa8b27ed57f5c6b60579b0bb AS build
RUN apk add --no-cache nodejs npm
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN npm ci && go generate ./...
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/sifeucr .

# Standalone binary for CI, migration jobs and Kubernetes.
FROM alpine:3@sha256:f59abe656b692d901e184be3445e084786602108599d2a51d229c0f5a48c71bf AS sifeucr
RUN apk upgrade --no-cache && apk add --no-cache ca-certificates
COPY --from=build /out/sifeucr /sifeucr
# Keep config discovery away from the extensionless /sifeucr executable.
WORKDIR /srv
USER 65532:65532
EXPOSE 3080
ENTRYPOINT ["/sifeucr"]

FROM nginx:stable-alpine@sha256:901e593c95dc85f5a729f5765516321366ef4ac401821431d408897e95287d0a
RUN apk upgrade --no-cache && apk add --no-cache ca-certificates su-exec tini \
    && addgroup -S -g 65532 sifeucr \
    && adduser -S -D -H -u 65532 -G sifeucr sifeucr

COPY --from=build /out/sifeucr /sifeucr
COPY deploy/nginx.conf /etc/nginx/conf.d/default.conf
COPY --chmod=755 deploy/entrypoint.sh /entrypoint.sh

WORKDIR /srv
ENV PORT=80
EXPOSE 80
STOPSIGNAL SIGTERM
ENTRYPOINT ["/sbin/tini", "--", "/entrypoint.sh"]
