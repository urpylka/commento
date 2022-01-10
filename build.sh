#!/bin/sh

docker buildx build --push \
    --tag urpylka/commento:latest \
    --platform linux/amd64,linux/arm64 .
