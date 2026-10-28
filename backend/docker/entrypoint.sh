#!/bin/sh
set -e

# Migrations run only in front of the API, so a cli container started from the
# same image does not race it. Set RUN_MIGRATIONS=false to run them as a separate step.
# The migrations are embedded in the cli binary.
if [ "$1" = "/app/api" ] && [ "${RUN_MIGRATIONS:-true}" = "true" ]; then
  echo "Running migrations..."
  /app/cli migrate up
fi

exec "$@"
