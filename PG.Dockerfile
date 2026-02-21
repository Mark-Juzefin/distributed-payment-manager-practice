FROM postgres:17

# pg_partman extension
RUN apt-get update && apt-get install -y --no-install-recommends \
      build-essential wget ca-certificates postgresql-server-dev-17

RUN wget "https://github.com/pgpartman/pg_partman/archive/refs/tags/v5.2.4.tar.gz" \
    && tar zxf v5.2.4.tar.gz && cd pg_partman-5.2.4 \
    && make \
    && make install \
     && rm -rf /var/lib/apt/lists/* v5.2.4.tar.gz pg_partman-5.2.4

# Patroni + etcd client
RUN apt-get update && apt-get install -y --no-install-recommends \
      python3 python3-pip python3-dev libpq-dev \
    && pip3 install --break-system-packages patroni[etcd3] psycopg2-binary \
    && rm -rf /var/lib/apt/lists/*

# Patroni config + post-bootstrap script
COPY config/patroni.yml /etc/patroni/patroni.yml
COPY scripts/post-bootstrap.sh /etc/patroni/post-bootstrap.sh
RUN chmod +x /etc/patroni/post-bootstrap.sh

# Entrypoint: ensure data dir exists with correct permissions, then start Patroni
COPY scripts/patroni-entrypoint.sh /usr/local/bin/patroni-entrypoint.sh
RUN chmod +x /usr/local/bin/patroni-entrypoint.sh

USER postgres
ENTRYPOINT ["patroni-entrypoint.sh"]
CMD ["/etc/patroni/patroni.yml"]
