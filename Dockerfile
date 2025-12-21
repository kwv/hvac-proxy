# Optimized for GoReleaser and fast local builds.
FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app
COPY hvac-proxy /app/hvac-proxy

EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/app/hvac-proxy"]