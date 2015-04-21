FROM postgres:9.4
ADD reset-db.sh /docker-entrypoint-initdb.d/
ADD vault.sql /db/
