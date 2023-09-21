#!/usr/bin/env bash

go run main.go --log-level debug &
if [[ "$OSTYPE" == "darwin"* ]]; then
sleep 5
fi
sleep 4
>&2 echo "Switching to INFO level"
curl -s -XPOST -d "INFO" "http://localhost:8081/loggers/level" > /dev/null
sleep 4
>&2 echo "Switching to WARN level"
curl -s -XPOST -d "WARN" "http://localhost:8081/loggers/level" > /dev/null
sleep 4
>&2 echo "Switching to ERROR level"
curl -s -XPOST -d "ERROR" "http://localhost:8081/loggers/level" > /dev/null
sleep 4
>&2 echo "Switching back to DEBUG level"
curl -s -XPOST -d "DEBUG" "http://localhost:8081/loggers/level" > /dev/null
sleep 4