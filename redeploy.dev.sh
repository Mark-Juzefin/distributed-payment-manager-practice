#!/bin/sh
docker-compose -f docker-compose.dev.yaml down --remove-orphans
docker-compose -f docker-compose.dev.yaml --profile dev up