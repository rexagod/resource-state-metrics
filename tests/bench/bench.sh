#!/usr/bin/env bash
# The script uses the following environment variables:
# - KUBESTATEMETRICS_DIR: The directory where kube-state-metrics is cloned.
# The following environment variables are optional:
# - KUBESTATEMETRICS_CUSTOMRESOURCESTATE_CONFIG: The path to the custom resource state configuration file.
# - GO: The path to the go binary.
# - KUBECONFIG: The path to the kubeconfig file.
# - KUBECTL: The path to the kubectl binary.
# - LOCAL_NAMESPACE: The namespace where the controller is deployed.
# - PROJECT_NAME: The name of the project.
set -uo pipefail

# If running in quiet mode, suppress all non-error logs.
if [ -n "${QUIET:-}" ]; then
  exec 1>/dev/null
fi

# Prepare prerequisites.
bench_file="/tmp/resourcestatemetrics_benchmark.txt"
rm "$bench_file" || true
retry_count=0
max_retries=10
REALPATH_KUBESTATEMETRICS_CUSTOMRESOURCESTATE_CONFIG=$(realpath "$KUBESTATEMETRICS_CUSTOMRESOURCESTATE_CONFIG")
date >> "$bench_file"
function terminate() { # Terminate dangling process in case of a manual interrupt.
  kill -9 "$PID"
  wait "$PID"
}
trap terminate SIGINT SIGTERM

# Run ResourceStateMetrics.
"$KUBECTL" scale deployment "$PROJECT_NAME" --replicas=0 -n "$LOCAL_NAMESPACE" || true

# Start the timer.
TInit=$(gdate +%s%3N)

# Run ResourceStateMetrics.
./"$PROJECT_NAME" --kubeconfig "$KUBECONFIG" &
PID=$!
echo -e "[$PID&]"

# Wait until the ports are open.
while ! nc -z localhost 9999; do sleep 1; done

# Set the time taken.
TTBuild=$(gdate +%s%3N)

# Query the metrics.
while ! curl -s http://localhost:9999/metrics | grep -q "kube_customresource_"; do
  sleep 1
  retry_count=$((retry_count + 1))
  if [ "$retry_count" -eq "$max_retries" ]; then
    echo "Failed to query metrics."
    exit 1
  fi
done

# Set the time taken.
TTQuery=$(gdate +%s%3N)

# Kill the background process.
kill -2 $PID
wait $PID

# Preserve the time differences.
# shellcheck disable=SC2129
echo -e "[RESOURCESTATEMETRICS]" >> "$bench_file"
echo -e "BUILD:\t$((TTBuild - TInit))ms" >> "$bench_file"
echo -e "RTT:\t$((TTQuery - TTBuild))ms" >> "$bench_file"

# If $KUBESTATEMETRICS_DIR is not set, or points to a non-directory, fail.
if [ -z "$KUBESTATEMETRICS_DIR" ] || [ ! -d "$KUBESTATEMETRICS_DIR" ]; then
 echo "KUBESTATEMETRICS_DIR is not set or does not point to a directory."
  exit 1
fi

# Set working directory to $KUBESTATEMETRICS_DIR.
cd "$KUBESTATEMETRICS_DIR" || exit 1

# Verify if we are actually in the right directory.
if [ "$(go list -m)" != "k8s.io/kube-state-metrics/v2" ]; then
  echo "Specified directory does not contain module k8s.io/kube-state-metrics/v2."
  exit 1
fi

# Update dependencies.
$GO mod tidy
$GO install

# Start timer.
TInit=$(gdate +%s%3N)

# Run KSM.
kube-state-metrics \
--kubeconfig "$KUBECONFIG" \
--custom-resource-state-config-file "$REALPATH_KUBESTATEMETRICS_CUSTOMRESOURCESTATE_CONFIG" \
--custom-resource-state-only &
PID=$!
echo -e "[$PID&]"

# Wait until the ports are open.
while ! nc -z localhost 8080; do sleep 1; done

# Set the time taken.
TTBuild=$(gdate +%s%3N)

# Query the metrics.
while ! curl -s http://localhost:8080/metrics | grep -q "kube_customresource_"; do
  sleep 1
  retry_count=$((retry_count + 1))
  if [ "$retry_count" -eq "$max_retries" ]; then
    echo "Failed to query metrics."
    exit 1
  fi
done

# Set the time taken.
TTQuery=$(gdate +%s%3N)

# Preserve the time differences.
# shellcheck disable=SC2129
echo -e "[CUSTOMRESOURCESTATE]" >> "$bench_file"
echo -e "BUILD:\t$((TTBuild - TInit))ms" >> "$bench_file"
echo -e "RTT:\t$((TTQuery - TTBuild))ms" >> "$bench_file"

# Set working directory to original directory.
cd - || exit 1

# Kill the background process.
kill -9 $PID
wait $PID

# Show the benchmark results.
cat "$bench_file"
