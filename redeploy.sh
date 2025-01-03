#!/bin/sh
docker-compose -f docker-compose.prod.yaml down --remove-orphans
docker-compose -f docker-compose.prod.yaml --profile prod up