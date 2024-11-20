#!/usr/bin/env bash

set -o pipefail

# Timeout duration in seconds
TIMEOUT=${TEST_TIMEOUT:-240}

# Start the controller in the background with a timeout.
timeout --signal SIGINT "$TIMEOUT" make setup apply apply-testdata local &
PID=$!
echo -e "\n[Running controller in background with PID: $PID]\n"

# Wait until telemetry is up.
while ! nc -z localhost "$RSM_MAIN_PORT"; do sleep 1; done

# Run tests.
echo -e "\n[Running tests with timeout: $TIMEOUT seconds]\n"
"$GO" test -count=1 -v -timeout "${TIMEOUT}s" -run "$TEST_RUN_PATTERN" -race "$TEST_PKG"
TEST_EXIT_CODE=$?

# Trap on SIGINT to terminate the controller.
function terminate() {
  kill -2 $PID
  wait $PID
}
trap terminate SIGINT

# Don't wait for the controller to finish and fail fast.
terminate

# Wait for signal.
wait $PID

# Cleanup.
make delete delete-testdata

# Exit with the test exit code.
exit $TEST_EXIT_CODE
