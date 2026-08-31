#!/bin/bash
docker rm exchange_check_app && docker rmi exchange_check_app:latest
docker build -t exchange_check_app:latest .
docker run -d \
  --name exchange_check_app \
  -p 8080:8080 \
  --restart=always \
  exchange_check_app:latest