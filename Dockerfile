FROM alpine:3.20
ARG TARGETARCH
COPY --chmod=0755 searchmiddleware-linux-${TARGETARCH} /usr/local/bin/searchmiddleware
ENTRYPOINT ["/usr/local/bin/searchmiddleware"]
