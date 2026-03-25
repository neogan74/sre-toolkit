#!/usr/bin/env sh
set -eu

ALERT_ANALYZER_BIN="${ALERT_ANALYZER_BIN:-../../../bin/alert-analyzer}"
VICTORIA_URL="${VICTORIA_URL:-http://localhost:8428}"
LOOKBACK="${LOOKBACK:-1h}"
TMP_OUTPUT="${TMP_OUTPUT:-/tmp/alert-analyzer-vm-smoke.json}"

echo "Running VictoriaMetrics compatibility smoke test"
echo "Binary: ${ALERT_ANALYZER_BIN}"
echo "URL: ${VICTORIA_URL}"

"${ALERT_ANALYZER_BIN}" analyze \
  --prometheus-url "${VICTORIA_URL}" \
  --lookback "${LOOKBACK}" \
  --show-flapping \
  --show-correlation \
  --show-temporal-patterns \
  --show-recommendations \
  --output json > "${TMP_OUTPUT}"

grep -q '"summary"' "${TMP_OUTPUT}"
grep -q '"frequency_analysis"' "${TMP_OUTPUT}"
grep -q '"recommendations"' "${TMP_OUTPUT}"
grep -q 'dead_rule' "${TMP_OUTPUT}"
grep -q '"temporal_patterns"' "${TMP_OUTPUT}"

echo "VictoriaMetrics compatibility smoke test passed"
