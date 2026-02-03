# Stress Tests

This directory contains stress tests for the KEDA HTTP Add-on. These tests are designed to validate the system's behavior under heavy and sustained load conditions.

## Overview

Stress tests differ from e2e tests in their focus:
- **E2E tests**: Verify functional correctness of features
- **Stress tests**: Verify system stability, performance, and scalability under extreme conditions

## Available Stress Tests

### 1. High Concurrency Stress Test (`high_concurrency_stress_test.go`)
Tests the system's ability to handle a large number of concurrent requests.

- **Load Pattern**: 100,000 requests with 100 concurrent connections
- **Purpose**: Validates that the system can scale up to maximum replicas and remain stable under high concurrent load
- **Duration**: ~10 minutes
- **Validates**:
  - Scaling to maximum replicas
  - System stability at maximum capacity
  - Proper scale-down after load removal

### 2. Sustained Load Stress Test (`sustained_load_stress_test.go`)
Tests the system's ability to handle continuous load over an extended period.

- **Load Pattern**: 500,000 requests with 50 concurrent connections over 10 minutes
- **Purpose**: Validates system stability during prolonged periods of sustained traffic
- **Duration**: ~15 minutes
- **Validates**:
  - Progressive scaling under sustained load
  - System stability over extended periods
  - Proper resource management during long-running operations
  - Graceful scale-down after sustained load

### 3. Rapid Scaling Stress Test (`rapid_scaling_stress_test.go`)
Tests the system's ability to handle rapid scale up and down cycles.

- **Load Pattern**: 3 cycles of rapid scale-up and scale-down (300,000 requests per cycle with 10 concurrent connections)
- **Purpose**: Validates system stability during frequent scaling operations
- **Duration**: ~20 minutes
- **Validates**:
  - Quick response to load spikes
  - Rapid scale-down when load drops
  - System stability across multiple scaling cycles
  - No resource leaks or degradation after multiple cycles

## Prerequisites

Same as e2e tests:
- [go](https://go.dev/)
- `kubectl` logged into a Kubernetes cluster
- Sufficient cluster resources to handle stress tests (recommended: at least 4 CPU cores and 8GB RAM)

## Running Stress Tests

### All Stress Tests

From the repository root:

```bash
make stress-test
```

This will:
1. Set up KEDA and the HTTP Add-on
2. Run all stress tests sequentially (they run one at a time to avoid resource contention)
3. Clean up all resources

### Setup Only

If you want to set up the environment without running tests:

```bash
make stress-test-setup
```

### Run Tests Without Setup

If the environment is already set up:

```bash
make stress-test-local
```

### Specific Stress Test

From the repository root:

```bash
cd tests
go test -v -tags stress ./stress/high_concurrency_stress_test.go
```

Or to run a specific test:

```bash
cd tests
go test -v -tags stress -run TestHighConcurrencyStress ./stress/
```

### Filter Tests by Name

Use the `STRESS_TEST_REGEX` environment variable:

```bash
STRESS_TEST_REGEX="high_concurrency" make stress-test-local
```

## Configuration

Stress test configuration can be adjusted in `tests/stress-run-all.go`:

- `concurrentTests`: Number of stress tests to run in parallel (default: 1)
- `testsTimeout`: Maximum time allowed for each test (default: 30m)
- `testsRetries`: Number of retries for failed tests (default: 2)

## Understanding Test Results

Each stress test logs its progress and validates specific conditions:

- **Scale-up validation**: Checks if the system scales to the expected number of replicas
- **Stability validation**: Ensures replicas remain stable under load
- **Scale-down validation**: Verifies proper cleanup after load removal

Tests use assertions to validate expected behavior. If any assertion fails, the test will fail and provide detailed error messages.

## Important Notes

1. **Resource Requirements**: Stress tests require more resources than e2e tests. Ensure your cluster has sufficient capacity.

2. **Test Duration**: Stress tests take longer to run than e2e tests (typically 10-15 minutes per test).

3. **Sequential Execution**: By default, stress tests run sequentially to avoid resource contention and provide more reliable results.

4. **Build Tag**: Stress tests use the `stress` build tag instead of `e2e`. This keeps them separate from regular e2e tests.

5. **Cleanup**: Each test cleans up its own resources. The test runner also performs a global cleanup.

## Adding New Stress Tests

To add a new stress test:

1. Create a new test file in `tests/stress/` with the naming pattern `*_stress_test.go`
2. Add the build tag at the top:
   ```go
   //go:build stress
   // +build stress
   ```
3. Follow the pattern from existing stress tests
4. Use unique namespace names to avoid conflicts
5. Include proper cleanup in your test
6. Test your stress test thoroughly before submitting

## Troubleshooting

### Test Timeouts
If tests timeout, you may need to:
- Increase `testsTimeout` in `stress-run-all.go`
- Increase cluster resources
- Adjust test parameters (reduce load or duration)

### Resource Constraints
If tests fail due to resource constraints:
- Check cluster capacity: `kubectl top nodes`
- Verify pod resource limits aren't too restrictive
- Consider reducing `maxReplicaCount` in tests

### Test Failures
If tests fail intermittently:
- Check for other workloads competing for resources
- Review logs from KEDA and HTTP Add-on components
- Increase wait times in the test if the cluster is slow to respond

## VS Code Configuration

To avoid IDE errors with the `stress` build tag, add to `.vscode/settings.json`:

```json
{
  "go.buildFlags": [
      "-tags=stress"
  ],
  "go.testTags": "stress",
}
```

Note: You may need to switch this back to `e2e` when working on e2e tests.
