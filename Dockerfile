FROM gcr.io/distroless/static:nonroot
COPY bot /bot
EXPOSE 8080 8443
ENTRYPOINT ["/bot"]
