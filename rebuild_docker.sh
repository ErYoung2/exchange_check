#!/bin/bash
docker run -d \
  --name exchange_check_app \
  -p 8080:8080 \
  --restart=always \
  exchange_check_app:latest