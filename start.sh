#!/bin/bash

# Ensure we are in the project root
cd "$(dirname "$0")"

# Check if .env file exists
if [ ! -f .env ]; then
    echo "Warning: .env file not found. Copying from .env.example if available or creating one."
    if [ -f .env.example ]; then
        cp .env.example .env
        echo "Created .env from .env.example. Please update it with your API key."
    else
        echo "Creating empty .env"
        touch .env
    fi
fi

echo "Starting Go Server..."
go run cmd/api/main.go
