FROM alpine:3.20
ARG TARGETARCH
COPY searchmiddleware-linux-${TARGETARCH} /usr/local/bin/searchmiddleware
ENTRYPOINT ["/usr/local/bin/searchmiddleware"]
