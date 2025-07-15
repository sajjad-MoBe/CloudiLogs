FROM cockroachdb/cockroach:v23.1.10
COPY init.sh /init.sh
RUN chmod +x /init.sh
ENTRYPOINT ["/init.sh"]
