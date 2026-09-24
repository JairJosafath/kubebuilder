# The test script builds a Linux binary locally using the existing Go cache.
FROM gcr.io/distroless/static:nonroot
COPY manager /manager
USER 65532:65532
ENTRYPOINT ["/manager"]
