FROM confluentinc/cp-kafka:7.3.2

COPY init.sh /init.sh
RUN chmod +x /init.sh

ENTRYPOINT ["/init.sh"]
