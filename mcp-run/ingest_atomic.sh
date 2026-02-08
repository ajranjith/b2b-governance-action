#!/bin/sh
# simulate atomic ingestion: rename incoming -> locked
echo "writing incoming" > /tmp/incoming.tmp
mv /tmp/incoming.tmp /tmp/incoming
mv /tmp/incoming /tmp/locked
