#!/bin/bash
echo "******CREATING DOCKER DATABASE******"

echo "starting postgres"
gosu postgres pg_ctl -w start

echo "create database"
gosu postgres createdb images

echo "initializing tables"
gosu postgres psql -h localhost -p 5432 -U postgres -d images -a -f /db/vault.sql

echo "stopping postgres"
gosu postgres pg_ctl stop

echo "stopped postgres"


echo ""
echo "******DOCKER DATABASE CREATED******"
