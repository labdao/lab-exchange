#!/bin/bash
CONTAINER_NAME="protein-binder-designer"
PLEX_JOB_INPUTS=$(cat user_input.json)
HOST_CACHE_DIR=$(pwd)/cache

docker build --rm -t $CONTAINER_NAME .

mkdir -p "$HOST_CACHE_DIR"
docker run -it --gpus=all \
-e PLEX_JOB_INPUTS="$PLEX_JOB_INPUTS" \
-e HF_HOME=/transformers_cache \
-v "$HOST_CACHE_DIR":/transformers_cache \
# -v "$(pwd)":/inputs \
-v "$(pwd)/outputs":/app/outputs \
$CONTAINER_NAME:latest