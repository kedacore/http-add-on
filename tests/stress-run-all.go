//go:build stress
// +build stress

package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"golang.org/x/sync/semaphore"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	"github.com/kedacore/http-add-on/tests/helper"
)

var (
	concurrentTests = 1
	testsTimeout    = "30m"
	testsRetries    = 2
)

type TestResult struct {
	TestCase string
	Passed   bool
	Tries    []string
}

func main() {
	ctx := context.Background()
	skipSetup := os.Getenv("SKIP_SETUP") == "true"
	onlySetup := os.Getenv("ONLY_SETUP") == "true"

	//
	// Install KEDA HTTP Add-on
	//
	if !skipSetup {
		installation := executeTest(ctx, "tests/utils/setup_test.go", "15m", 1)
		fmt.Print(installation.Tries[0])
		if !installation.Passed {
			uninstallKeda(ctx)
			os.Exit(1)
		}
	}

	if !onlySetup {
		//
		// Execute stress tests
		//
		testResults := executeStressTests(ctx)

		//
		// Uninstall KEDA
		//
		if !skipSetup {
			passed := uninstallKeda(ctx)
			if !passed {
				os.Exit(1)
			}
		}

		//
		// Generate execution outcome
		//
		exitCode := evaluateExecution(testResults)

		os.Exit(exitCode)
	}
}

func executeTest(ctx context.Context, file string, timeout string, tries int) TestResult {
	result := TestResult{
		TestCase: file,
		Passed:   false,
		Tries:    []string{},
	}
	for i := 1; i <= tries; i++ {
		fmt.Printf("Executing %s, try '%d'\n", file, i)
		var cmd *exec.Cmd
		// Use stress build tag for stress tests
		if strings.Contains(file, "stress") {
			cmd = exec.CommandContext(ctx, "go", "test", "-v", "-tags", "stress", "-timeout", timeout, file)
		} else {
			cmd = exec.CommandContext(ctx, "go", "test", "-v", "-tags", "e2e", "-timeout", timeout, file)
		}
		stdout, err := cmd.CombinedOutput()
		logFile := fmt.Sprintf("%s.%d.log", file, i)
		fileError := os.WriteFile(logFile, stdout, 0644)
		if fileError != nil {
			fmt.Printf("Execution of %s, try '%d' has failed writing the logs : %s\n", file, i, fileError)
		}
		result.Tries = append(result.Tries, string(stdout))
		if err == nil {
			fmt.Printf("Execution of %s, try '%d' has passed\n", file, i)
			result.Passed = true
			break
		}
		fmt.Printf("Execution of %s, try '%d' has failed: %s \n", file, i, err)
	}
	return result
}

func getStressTestFiles() []string {
	testFiles := []string{}

	stressRegex := os.Getenv("STRESS_TEST_REGEX")
	if stressRegex == "" {
		stressRegex = ".*"
	}
	regex, err := regexp.Compile(stressRegex)
	if err != nil {
		fmt.Printf("Error compiling regex: %s\n", err)
		os.Exit(1)
	}

	err = filepath.Walk("tests/stress",
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if strings.HasSuffix(info.Name(), "_test.go") &&
				regex.MatchString(info.Name()) {
				testFiles = append(testFiles, path)
			}
			return nil
		})

	if err != nil {
		return []string{}
	}

	// Randomize the test execution order
	rand.Shuffle(len(testFiles), func(i, j int) {
		testFiles[i], testFiles[j] = testFiles[j], testFiles[i]
	})

	return testFiles
}

func executeStressTests(ctx context.Context) []TestResult {
	sem := semaphore.NewWeighted(int64(concurrentTests))
	mutex := &sync.RWMutex{}
	testResults := []TestResult{}

	//
	// Execute stress tests
	//
	testFiles := getStressTestFiles()
	fmt.Printf("Found %d stress test files to execute\n", len(testFiles))

	for _, testFile := range testFiles {
		if err := sem.Acquire(ctx, 1); err != nil {
			fmt.Printf("Failed to acquire semaphore: %v", err)
			uninstallKeda(ctx)
			os.Exit(1)
		}

		go func(file string) {
			defer sem.Release(1)
			testExecution := executeTest(ctx, file, testsTimeout, testsRetries)
			mutex.Lock()
			testResults = append(testResults, testExecution)
			mutex.Unlock()
		}(testFile)
	}

	// Wait until all tests end
	if err := sem.Acquire(ctx, int64(concurrentTests)); err != nil {
		log.Printf("Failed to acquire semaphore: %v", err)
	}

	//
	// Print regular logs
	//
	for _, result := range testResults {
		status := "failed"
		if result.Passed {
			status = "passed"
		}
		fmt.Printf("%s has %s after %d tries \n", result.TestCase, status, len(result.Tries))
		for index, log := range result.Tries {
			fmt.Printf("try number %d\n", index+1)
			fmt.Println(log)
		}
	}

	kubeConfig, _ := config.GetConfig()
	kubeClient, _ := kubernetes.NewForConfig(kubeConfig)

	kedaLogs, err := helper.FindPodLogs(kubeClient, helper.KEDANamespace, "app=keda-operator")
	if err == nil {
		fmt.Println(">>> KEDA Operator log <<<")
		fmt.Println(kedaLogs)
		fmt.Println("##############################################")
		fmt.Println("##############################################")
	}

	operatorLogs, err := helper.FindPodLogs(kubeClient, helper.KEDANamespace, "app.kubernetes.io/instance=operator")
	if err == nil {
		fmt.Println(">>> HTTP Add-on Operator log <<<")
		fmt.Println(operatorLogs)
		fmt.Println("##############################################")
		fmt.Println("##############################################")
	}

	interceptorLogs, err := helper.FindPodLogs(kubeClient, helper.KEDANamespace, "app.kubernetes.io/instance=interceptor")
	if err == nil {
		fmt.Println(">>> HTTP Add-on Interceptor log <<<")
		fmt.Println(interceptorLogs)
		fmt.Println("##############################################")
		fmt.Println("##############################################")
	}

	scalerLogs, err := helper.FindPodLogs(kubeClient, helper.KEDANamespace, "app.kubernetes.io/instance=external-scaler")
	if err == nil {
		fmt.Println(">>> HTTP Add-on Scaler log <<<")
		fmt.Println(scalerLogs)
		fmt.Println("##############################################")
		fmt.Println("##############################################")
	}

	return testResults
}

func uninstallKeda(ctx context.Context) bool {
	removal := executeTest(ctx, "tests/utils/cleanup_test.go", "15m", 1)
	fmt.Print(removal.Tries[0])
	return removal.Passed
}

func evaluateExecution(testResults []TestResult) int {
	passSummary := []string{}
	failSummary := []string{}
	exitCode := 0

	for _, result := range testResults {
		if !result.Passed {
			message := fmt.Sprintf("\tExecution of %s, has failed after %d tries", result.TestCase, len(result.Tries))
			failSummary = append(failSummary, message)
			exitCode = 1
			continue
		}
		message := fmt.Sprintf("\tExecution of %s, has passed after %d tries", result.TestCase, len(result.Tries))
		passSummary = append(passSummary, message)
	}

	fmt.Println("##############################################")
	fmt.Println("##############################################")
	fmt.Println("STRESS TEST EXECUTION SUMMARY")
	fmt.Println("##############################################")
	fmt.Println("##############################################")

	if len(passSummary) > 0 {
		fmt.Println("Passed stress tests:")
		for _, message := range passSummary {
			fmt.Println(message)
		}
	}

	if len(failSummary) > 0 {
		fmt.Println("Failed stress tests:")
		for _, message := range failSummary {
			fmt.Println(message)
		}
	}

	return exitCode
}
