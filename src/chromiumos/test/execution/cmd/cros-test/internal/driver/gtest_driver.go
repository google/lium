// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package driver implements drivers to execute tests.
package driver

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"chromiumos/test/execution/cmd/cros-test/internal/device"
	"chromiumos/test/execution/errors"

	"go.chromium.org/chromiumos/config/go/test/api"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// GtestDriver runs gtest and report its results.
type GtestDriver struct {
	// logger provides logging service.
	logger *log.Logger
}

// NewGtestDriver creates a new driver to run tests.
func NewGtestDriver(logger *log.Logger) *GtestDriver {
	return &GtestDriver{
		logger: logger,
	}
}

// Name returns the name of the driver.
func (td *GtestDriver) Name() string {
	return "gtest"
}

// gtestFailure represents gtest failure information
// ...
//
//		"failures": [
//				  {
//					"failure": "temp.cc:16\nExpected equality of these values:\n  0\n  1",
//					"type": ""
//				  }
//	 ]
type gtestFailure struct {
	Failure string
	Type    string
}

// gtestCase represents a test case execution
// ...
//
//	  {
//		"name": "Negative",
//		"status": "RUN",
//		"result": "COMPLETED",
//		"timestamp": "2021-11-24T15:23:23Z",
//		"time": "0s",
//		"classname": "StubTest",
//		"failures": [
//		  {
//			"failure": "temp.cc:16\nExpected equality of these values:\n  0\n  1",
//			"type": ""
//		  }
//		]
//	  }
type gtestCase struct {
	Name      string
	Status    string
	Result    string
	Timestamp string
	Time      string
	Classname string
	Failures  []gtestFailure
}

// gtestSuite represents a gtest suite
// ...
//
//	  {
//		"name": "StubTest",
//		"tests": 2,
//		"failures": 1,
//		"disabled": 0,
//		"errors": 0,
//		"timestamp": "2021-11-24T15:23:23Z",
//		"time": "0s",
//		"testsuite": [
//		  {
//			"name": "Positive",
//			"status": "RUN",
//			"result": "COMPLETED",
//			"timestamp": "2021-11-24T15:23:23Z",
//			"time": "0s",
//			"classname": "StubTest"
//		  },
//		  {
//			"name": "Negative",
//			"status": "RUN",
//			"result": "COMPLETED",
//			"timestamp": "2021-11-24T15:23:23Z",
//			"time": "0s",
//			"classname": "StubTest",
//			"failures": [
//			  {
//				"failure": "temp.cc:16\nExpected equality of these values:\n  0\n  1",
//				"type": ""
//			  }
//			]
//		  }
//		]
//	  }
type gtestSuite struct {
	Name      string
	Tests     int
	Failures  int
	Disabled  int
	Errors    int
	Time      string
	TestSuite []gtestCase
}

// gtestResult represents the entire gtest execution
// ...
// {
//
//	"tests": 2,
//	"failures": 1,
//	"disabled": 0,
//	"errors": 0,
//	"timestamp": "2021-11-24T15:23:23Z",
//	"time": "0s",
//	"name": "AllTests",
//	"testsuites": [
//	  {
//		"name": "StubTest",
//		"tests": 2,
//		"failures": 1,
//		"disabled": 0,
//		"errors": 0,
//		"timestamp": "2021-11-24T15:23:23Z",
//		"time": "0s",
//		"testsuite": [
//		  {
//			"name": "Positive",
//			"status": "RUN",
//			"result": "COMPLETED",
//			"timestamp": "2021-11-24T15:23:23Z",
//			"time": "0s",
//			"classname": "StubTest"
//		  },
//		  {
//			"name": "Negative",
//			"status": "RUN",
//			"result": "COMPLETED",
//			"timestamp": "2021-11-24T15:23:23Z",
//			"time": "0s",
//			"classname": "StubTest",
//			"failures": [
//			  {
//				"failure": "temp.cc:16\nExpected equality of these values:\n  0\n  1",
//				"type": ""
//			  }
//			]
//		  }
//		]
//	  }
//	]
//
// }
type gtestResult struct {
	Tests      int
	Failures   int
	Disabled   int
	Errors     int
	Timestamp  string
	Time       string
	Name       string
	TestSuites []gtestSuite
}

// Data needed to convert a gtestResult into ResultMetadata
type executionData struct {
	startTime time.Time
	duration  int64
	reasons   []string
}

func newExecutionData(startTime time.Time, duration int64, reasons []string) *executionData {
	return &executionData{
		startTime: startTime,
		duration:  duration,
		reasons:   reasons,
	}
}

// logCmd logs a remote command run through a DUT server
func logCmd(logger *log.Logger, cmd *api.ExecCommandRequest, resp *api.ExecCommandResponse) {
	logger.Printf("cmd '%v', args '%v'", cmd.Command, cmd.Args)
	logger.Printf("[status]:\n%v", resp.ExitInfo.Status)
	logger.Printf("[stdout]:\n%v", string(resp.Stdout))
	logger.Printf("[stderr]:\n%v", string(resp.Stderr))
	logger.Printf("[error]:\n%v", string(resp.ExitInfo.ErrorMessage))
}

const invalidDuration int64 = 0

// testResult returns list of reasons if TC failed, empty list otherwise.
func testResult(testCaseName string, defaultStartTime time.Time, result *gtestResult) *executionData {
	// Example results file (showing a pass and a fail).
	// Right now, only one result is expected, either pass or fail, this is just an
	// example.
	//
	// {
	// 	"tests": 2,
	// 	"failures": 1,
	// 	"disabled": 0,
	// 	"errors": 0,
	// 	"timestamp": "2021-11-24T15:23:23Z",
	// 	"time": "0s",
	// 	"name": "AllTests",
	// 	"testsuites": [
	// 	  {
	// 		"name": "StubTest",
	// 		"tests": 2,
	// 		"failures": 1,
	// 		"disabled": 0,
	// 		"errors": 0,
	// 		"timestamp": "2021-11-24T15:23:23Z",
	// 		"time": "0s",
	// 		"testsuite": [
	// 		  {
	// 			"name": "Positive",
	// 			"status": "RUN",
	// 			"result": "COMPLETED",
	// 			"timestamp": "2021-11-24T15:23:23Z",
	// 			"time": "0s",
	// 			"classname": "StubTest"
	// 		  },
	// 		  {
	// 			"name": "Negative",
	// 			"status": "RUN",
	// 			"result": "COMPLETED",
	// 			"timestamp": "2021-11-24T15:23:23Z",
	// 			"time": "0s",
	// 			"classname": "StubTest",
	// 			"failures": [
	// 			  {
	// 				"failure": "temp.cc:16\nExpected equality of these values:\n  0\n  1",
	// 				"type": ""
	// 			  }
	// 			]
	// 		  }
	// 		]
	// 	  }
	// 	]
	// }
	//
	// Test case name is "classname"."name", in this case "StubTest.Positive"
	//
	// Var defaultStartTime is used when startTime cannot be read from the gtest output.

	// First make sure we have results, should have one suite and one case.
	if len(result.TestSuites) != 1 || len(result.TestSuites[0].TestSuite) != 1 {
		return newExecutionData(defaultStartTime, invalidDuration, []string{"no test results found"})
	}

	// Check that the classname and casename are as expected.
	// testCaseName should be of format '<classname>.<casename>'.
	nameParts := strings.SplitN(testCaseName, ".", 2)
	if len(nameParts) != 2 {
		return newExecutionData(defaultStartTime, invalidDuration, []string{fmt.Sprintf("unexpected testCaseName, got: '%v', want: format '<className>.<caseName>'", testCaseName)})
	}
	className, caseName := nameParts[0], nameParts[1]

	testCase := result.TestSuites[0].TestSuite[0]
	if testCase.Classname != className {
		return newExecutionData(defaultStartTime, invalidDuration, []string{fmt.Sprintf("mismatched classname, got: '%v', want: '%v'", testCase.Classname, caseName[0])})
	}

	if testCase.Name != caseName {
		return newExecutionData(defaultStartTime, invalidDuration, []string{fmt.Sprintf("mismatched case name, got: '%v', want: '%v'", testCase.Name, caseName[1])})
	}

	// Check that status and result are as expected.
	status := strings.ToLower(testCase.Status)
	if status != "run" {
		return newExecutionData(defaultStartTime, invalidDuration, []string{fmt.Sprintf("mismatched case status, got: '%v', want: 'run'", status)})
	}

	runResult := strings.ToLower(testCase.Result)
	if runResult != "completed" {
		return newExecutionData(defaultStartTime, invalidDuration, []string{fmt.Sprintf("mismatched case result, got: '%v', want: 'completed", runResult)})
	}

	startTime, err := time.Parse(time.RFC3339, result.TestSuites[0].TestSuite[0].Timestamp)
	if err != nil {
		return newExecutionData(defaultStartTime, invalidDuration, []string{fmt.Sprintf("Error parsing test start time, value '%v', err: %v", result.TestSuites[0].TestSuite[0].Timestamp, err)})
	}

	var duration int64
	if count, err := fmt.Sscanf(result.TestSuites[0].TestSuite[0].Time, "%ds", &duration); err != nil || count != 1 {
		return newExecutionData(defaultStartTime, invalidDuration, []string{fmt.Sprintf("Error parsing test duration, original value '%v', err: %v", result.TestSuites[0].TestSuite[0].Time, err)})
	}

	// Grab any failures.
	if len(testCase.Failures) > 0 {
		var reasons []string

		for _, failure := range testCase.Failures {
			reasons = append(reasons, fmt.Sprintf("failure: '%v', type: '%v'", failure.Failure, failure.Type))
		}

		return newExecutionData(startTime, duration, reasons)
	}

	// Test passed supposedly, make sure no unexpected failures or errors
	if result.Failures != 0 || result.Errors != 0 || result.TestSuites[0].Failures != 0 || result.TestSuites[0].Disabled != 0 {
		return newExecutionData(startTime, duration, []string{"unexpected errors in gtest results"})
	}

	return newExecutionData(startTime, duration, nil)
}

// buildTestCaseResults builds the api.TestCaseResult object for a given test result.
// If err is populated, and ERROR status will be returned.
func buildTestCaseResults(tcID string, results *executionData) *api.TestCaseResult {
	tcResult := new(api.TestCaseResult)

	tcResult.TestHarness = &api.TestHarness{TestHarnessType: &api.TestHarness_Gtest_{Gtest: &api.TestHarness_Gtest{}}}
	tcResult.TestCaseId = &api.TestCase_Id{Value: tcID}
	tcResult.StartTime = timestamppb.New(results.startTime)
	tcResult.Duration = &durationpb.Duration{Seconds: results.duration}

	// Test passed if no reasons specified
	if len(results.reasons) == 0 {
		tcResult.Verdict = &api.TestCaseResult_Pass_{Pass: &api.TestCaseResult_Pass{}}
	} else {
		tcResult.Verdict = &api.TestCaseResult_Fail_{Fail: &api.TestCaseResult_Fail{}}
		tcResult.Reason = strings.Join(results.reasons, "\n")
	}

	return tcResult
}

// runGtestCmd executes a test on the DUT.
// reasons for failure, if any. Empty reasons means command passed.
func runGtestCmd(ctx context.Context, logger *log.Logger, dut api.DutServiceClient, test *api.TestCaseMetadata) *executionData {
	var err error

	targetBinLocation := test.TestCaseExec.GetTestHarness().GetGtest().GetTargetBinLocation()
	startTime := time.Now()
	outFileName := fmt.Sprintf("/tmp/%v-%d.json", test.TestCase.Id.Value, startTime.Unix())

	// Execute the gtest on the DUT.
	cmdArgs := []string{
		fmt.Sprintf("--gtest_output=json:%v", outFileName),
		fmt.Sprintf("--gtest_filter=%v", test.TestCase.Name),
	}
	cmdExec := api.ExecCommandRequest{
		Command: targetBinLocation,
		Args:    cmdArgs,
	}

	var client api.DutService_ExecCommandClient
	if client, err = dut.ExecCommand(ctx, &cmdExec); err != nil {
		return newExecutionData(startTime, invalidDuration, []string{fmt.Sprintf("failed to exec command on DUT: %v", err)})
	}

	var resp *api.ExecCommandResponse
	if resp, err = client.Recv(); err != nil {
		return newExecutionData(startTime, invalidDuration, []string{fmt.Sprintf("failed to get command results: %v", err)})
	}

	logCmd(logger, &cmdExec, resp)

	// Gtest should return 0 or 1 if tests ran, anything else should be
	// looked at as an execution/infra failure.
	//
	// Occassionally, gtest will receive a bad arg and return a zero exit
	// code but not actually run the tests. Because some tests might have stdout
	// output, the best way to catch this is to log the command and the driver
	// will fail when it tries to parse the results file, which won't exist.
	if resp.ExitInfo.Status != 0 && resp.ExitInfo.Status != 1 {
		return newExecutionData(startTime, invalidDuration, []string{fmt.Sprintf("unexpected failure: stderr: %v, err: %v", string(resp.Stderr), resp.ExitInfo.ErrorMessage)})
	}

	// Test has passed, now get the results.
	cmdExec = api.ExecCommandRequest{
		Command: "cat",
		Args:    []string{outFileName},
	}

	if client, err = dut.ExecCommand(ctx, &cmdExec); err != nil {
		return newExecutionData(startTime, invalidDuration, []string{fmt.Sprintf("failed to exec command on DUT: %v", err)})
	}

	if resp, err = client.Recv(); err != nil {
		return newExecutionData(startTime, invalidDuration, []string{fmt.Sprintf("failed to get command results: %v", err)})
	}

	logCmd(logger, &cmdExec, resp)

	if resp.ExitInfo.Status != 0 {
		return newExecutionData(startTime, invalidDuration, []string{fmt.Sprintf("non-zero exit code (%d) reading test results:\nstderr:%v\nerr:%v",
			resp.ExitInfo.Status,
			string(resp.Stderr),
			resp.ExitInfo.ErrorMessage)})
	}

	// Build the test results struct.
	var gtestResults gtestResult
	if err = json.Unmarshal(resp.Stdout, &gtestResults); err != nil {
		return newExecutionData(startTime, invalidDuration, []string{fmt.Sprintf("failed to parse gtest json data: %v", err)})
	}

	return testResult(test.TestCase.Name, startTime, &gtestResults)
}

// RunTests drives a test framework to execute tests.
func (td *GtestDriver) RunTests(ctx context.Context, resultsDir string, req *api.CrosTestRequest, tlwAddr string, tests []*api.TestCaseMetadata) (*api.CrosTestResponse, error) {
	var err error
	var testCaseResults []*api.TestCaseResult

	// Setup dut connection to be able to run the tests and get results.
	var dutInfo *device.DutInfo
	if dutInfo, err = device.FillDUTInfo(req.Primary, ""); err != nil {
		return nil, errors.NewStatusError(errors.InvalidArgument,
			fmt.Errorf("cannot get address from primary device: %v", dutInfo))
	}

	var primaryDutConn *grpc.ClientConn
	if primaryDutConn, err = grpc.Dial(dutInfo.DutServer, grpc.WithInsecure()); err != nil {
		return nil, errors.NewStatusError(errors.InvalidArgument,
			fmt.Errorf("cannot create connection with primary device: %v, address: %v", req.Primary, dutInfo.DutServer))
	}
	defer primaryDutConn.Close()

	dut := api.NewDutServiceClient(primaryDutConn)

	for _, test := range tests {
		results := runGtestCmd(ctx, td.logger, dut, test)

		tcResult := buildTestCaseResults(test.TestCase.Id.Value, results)
		testCaseResults = append(testCaseResults, tcResult)
	}
	return &api.CrosTestResponse{TestCaseResults: testCaseResults}, nil
}