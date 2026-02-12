## Prerequisites

- [go](https://go.dev/)
- `kubectl` logged into a Kubernetes cluster.

## Test Types

This directory contains two types of tests:

### E2E Tests
End-to-end tests that verify functional correctness of the HTTP Add-on features. See below for running instructions.

### Stress Tests
Performance and scalability tests that validate system behavior under heavy load. These are located in the `stress/` subdirectory. See [stress/README.md](stress/README.md) for details on running stress tests.

## Running tests:

### All tests

Make sure that you are in `tests` directory.

```bash
go test -v -tags e2e ./utils/setup_test.go        # Only needs to be run once.
go test -v -tags e2e ./checks/...
go test -v -tags e2e ./utils/cleanup_test.go      # Skip if you want to keep testing.
```

### Specific test

```bash
go test -v -tags e2e ./checks/internal_service/internal_service_test.go # Assumes that setup has been run before
```

> **Note**
> On macOS you might need to set following environment variable in order to run the tests: `GOOS="darwin"`
>
> eg. `GOOS="darwin" go test -v tags e2e ...`

Refer to [this](https://pkg.go.dev/testing) for more information about testing in `Go`.

## E2E Test Setup

The test script will run in 3 phases:

- **Setup:** This is done in [`utils/setup_test.go`](utils/setup_test.go). If you're adding any tests to the install / setup process, you need to add it to this file. `utils/setup_test.go` deploys KEDA components to the `keda` namespace.

    After `utils/setup_test.go` is done, we expect to have KEDA components setup in the `keda` namespace.

- **Tests:** Currently there are only scaler tests in `tests/checks/`. Each test is kept in its own package. This is to prevent conflicting variable declarations for commonly used variables (**ex -** `testNamespace`). Individual scaler tests are run
in parallel, but tests within a file can be run in parallel or in series. More about tests below.

- **Global cleanup:** This is done in [`utils/cleanup_test.go`](utils/cleanup_test.go). It cleans up all the resources created in `utils/setup_test.go`.

> **Note**
> Your IDE might give you errors upon trying to import certain packages that use the `e2e` build tag. To overcome this, you will need to specify in your IDE settings to use the `e2e` build tag.
>
> As an example, in VSCode, it can be achieved by creating a `.vscode` directory within the project directory (if not present) and creating a `settings.json` file in that directory (or updating it) with the following content:
> ```json
> {
>   "go.buildFlags": [
>       "-tags=e2e"
>   ],
>   "go.testTags": "e2e",
> }
> ```

## Adding tests

- Tests are written using `Go`'s default [`testing`](https://pkg.go.dev/testing) framework, and [`testify`](https://pkg.go.dev/github.com/stretchr/testify).
- Each e2e test should be in its own package, **ex -** `checks/internal_service/internal_service_test.go`, etc
- Each test file is expected to do its own setup and clean for resources.

#### ⚠⚠ Important: ⚠⚠
>
> - Even though the cleaning of resources is expected inside each e2e test file, all test namespaces
> (namespaces with label type=e2e) are  cleaned up to ensure not having dangling resources after global e2e
> execution finishes. To not break this behaviour, it's mandatory to use the `CreateNamespace(t *testing.T, kc *kubernetes.Clientset, nsName string)` function from [`helper.go`](helper/helper.go), instead of creating them manually.

#### ⚠⚠ Important: ⚠⚠
> - `Go` code can panic when performing forbidden operations such as accessing a nil pointer, or from code that
> manually calls `panic()`. A function that `panics` passes the `panic` up the stack until program execution stops
> or it is recovered using `recover()` (somewhat similar to `try-catch` in other languages).
> - If a test panics, and is not recovered, `Go` will stop running the file, and no further tests will be run. This can
> cause issues with clean up of resources.
> - Ensure that you are not executing code that can lead to `panics`. If you think that there's a chance the test might
> panic, call `recover()`, and cleanup the created resources.
> - Read this [article](https://go.dev/blog/defer-panic-and-recover) for understanding more about `panic` and `recover` in `Go`.

#### Notes

- You can see [`internal_service_test.go`](checks/internal_service/internal_service_test.go) for a full example.
- All tests must have the `// +build e2e` build tag.
- Refer [`helper.go`](helper/helper.go) for various helper methods available to use in your tests.
- Prefer using helper methods or `k8s` libraries in `Go` over manually executing `shell` commands. Only if the task
you're trying to achieve is too complicated or tedious using above, use `ParseCommand` or `ExecuteCommand` from `helper.go`
for executing shell commands.
- Ensure, ensure, ensure that you're cleaning up resources.
- You can use `VS Code` for easily debugging your tests.
