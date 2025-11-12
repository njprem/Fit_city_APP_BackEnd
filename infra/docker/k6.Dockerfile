FROM grafana/k6:0.49.0

USER root
RUN apk add --no-cache imagemagick
COPY infra/docker/k6-entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh
USER 1000

ENTRYPOINT ["/entrypoint.sh"]
CMD ["run"]
