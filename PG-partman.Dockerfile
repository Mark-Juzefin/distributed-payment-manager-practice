FROM postgres:17

# pg_partman extension only — no Patroni
RUN apt-get update && apt-get install -y --no-install-recommends \
      build-essential wget ca-certificates postgresql-server-dev-17 \
    && wget "https://github.com/pgpartman/pg_partman/archive/refs/tags/v5.2.4.tar.gz" \
    && tar zxf v5.2.4.tar.gz && cd pg_partman-5.2.4 \
    && make \
    && make install \
    && rm -rf /var/lib/apt/lists/* v5.2.4.tar.gz pg_partman-5.2.4
