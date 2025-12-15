# binaries build
FROM docker.io/golang:1.25-alpine AS build

# hadolint ignore=DL3018
RUN ["apk", "add", "--no-cache", "make", "bash", "jq", "git"]
WORKDIR /src
COPY go.mod go.sum ./
RUN ["go", "mod", "download"]
COPY . ./
RUN ["make", "build"]
RUN echo "metricz-exporter:x:1000:1000:metricz-exporter:/data:/sbin/nologin" > ./passwd && \
    echo "metricz-exporter:x:1000:" > ./group

# create final root fs
FROM scratch AS root-fs

COPY --from=build /src/group /etc/group
COPY --from=build /src/passwd /etc/passwd
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /src/build/metricz-exporter /bin/metricz-exporter
WORKDIR /maps

# final binaries image
FROM scratch

USER 1000
WORKDIR /metricz
ENV PATH=/bin \
    METRICZ_CONFIG=/metricz/config.yaml \
    METRICZ_CONFIG_INIT=true \
    METRICZ_GEOIP_PATH=/metricz/metricz-city.mmdb \
    METRICZ_LOG_LEVEL=info
COPY --from=root-fs --chown=1000:1000 / /
ENTRYPOINT ["/bin/metricz-exporter"]
