// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"bytes"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"testing"

	"github.com/golang/protobuf/jsonpb"
	"github.com/google/go-cmp/cmp"
	_go "go.chromium.org/chromiumos/config/go"
	"go.chromium.org/chromiumos/config/go/test/api"
	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"
)

func TestReadInput(t *testing.T) {
	expReq := &api.CrosTestRequest{
		TestSuites: []*api.TestSuite{
			{
				Name: "suite1",
				Spec: &api.TestSuite_TestCaseIds{
					TestCaseIds: &api.TestCaseIdList{
						TestCaseIds: []*api.TestCase_Id{
							{
								Value: "example.Pass",
							},
							{
								Value: "example.Fail",
							},
						},
					},
				},
			},
			{
				Name: "suite2",
				Spec: &api.TestSuite_TestCaseTagCriteria_{
					TestCaseTagCriteria: &api.TestSuite_TestCaseTagCriteria{
						Tags: []string{"group:meta"},
					},
				},
			},
		},
		Primary: &api.CrosTestRequest_Device{
			Dut: &labapi.Dut{
				Id: &labapi.Dut_Id{Value: "Dut1"},
				DutType: &labapi.Dut_Chromeos{
					Chromeos: &labapi.Dut_ChromeOS{
						Ssh: &labapi.IpEndpoint{Address: "127.0.0.1", Port: 2222},
					},
				},
			},
		},
		Companions: []*api.CrosTestRequest_Device{
			{
				Dut: &labapi.Dut{
					Id: &labapi.Dut_Id{Value: "CompanionDut1"},
					DutType: &labapi.Dut_Chromeos{
						Chromeos: &labapi.Dut_ChromeOS{
							Ssh: &labapi.IpEndpoint{Address: "127.0.0.1", Port: 2223},
						},
					},
				},
			},
		},
	}

	m := jsonpb.Marshaler{}
	encodedData, err := m.MarshalToString(expReq)
	if err != nil {
		t.Fatal("Failed to marshall request")
	}
	td, err := ioutil.TempDir("", "cros-test_TestReadInput_*")
	if err != nil {
		t.Fatal("Failed to create temporary dictectory: ", err)
	}

	defer os.RemoveAll(td)
	fn := filepath.Join(td, "t.json")
	if err := ioutil.WriteFile(fn, []byte(encodedData), 0644); err != nil {
		t.Fatalf("Failed to write file %v: %v", fn, err)
	}
	ioutil.WriteFile("/tmp/t.json", []byte(encodedData), 0644)
	req, err := readInput(fn)
	if err != nil {
		t.Fatalf("Failed to read input file %v: %v", fn, err)
	}
	if diff := cmp.Diff(req, expReq, cmp.AllowUnexported(api.CrosTestRequest{})); diff != "" {
		t.Errorf("Got unexpected request from readInput (-got +want):\n%s", diff)
	}
}

func TestWriteOutput(t *testing.T) {
	resultDir := "/tmp/tast/results"
	tests := []string{
		"PassedTest",
		"FailedTest",
		"SkippedTest",
	}
	expectedRspn := api.CrosTestResponse{
		TestCaseResults: []*api.TestCaseResult{
			{
				TestCaseId: &api.TestCase_Id{Value: tests[0]},
				ResultDirPath: &_go.StoragePath{
					HostType: _go.StoragePath_LOCAL,
					Path:     filepath.Join(resultDir, "tests", tests[0]),
				},
				Verdict: &api.TestCaseResult_Pass_{Pass: &api.TestCaseResult_Pass{}},
			},
			{
				TestCaseId: &api.TestCase_Id{Value: tests[1]},
				ResultDirPath: &_go.StoragePath{
					HostType: _go.StoragePath_LOCAL,
					Path:     filepath.Join(resultDir, "tests", tests[1]),
				},
				Verdict: &api.TestCaseResult_Fail_{Fail: &api.TestCaseResult_Fail{}},
			},
			{
				TestCaseId: &api.TestCase_Id{Value: tests[2]},
				ResultDirPath: &_go.StoragePath{
					HostType: _go.StoragePath_LOCAL,
					Path:     filepath.Join(resultDir, "tests", tests[2]),
				},
				Verdict: &api.TestCaseResult_Skip_{Skip: &api.TestCaseResult_Skip{}},
				Reason:  "Test is skipped",
			},
		},
	}
	td, err := ioutil.TempDir("", "faketestrunner_TestWriteOutput_*")
	if err != nil {
		t.Fatal("Failed to create temporary dictectory: ", err)
	}
	defer os.RemoveAll(td)
	fn := filepath.Join(td, "t.json")
	if err := writeOutput(fn, &expectedRspn); err != nil {
		t.Fatalf("Failed to write file %v: %v", fn, err)
	}
	f, err := os.Open(fn)
	if err != nil {
		t.Fatalf("Failed to read response from file %v: %v", fn, err)
	}
	rspn := api.CrosTestResponse{}
	if err := jsonpb.Unmarshal(f, &rspn); err != nil {
		t.Fatalf("Failed to unmarshall data from file %v: %v", fn, err)
	}
	if diff := cmp.Diff(rspn, expectedRspn, cmp.AllowUnexported(api.CrosTestResponse{})); diff != "" {
		t.Errorf("Got unexpected data from writeOutput (-got +want):\n%s", diff)
	}
}

var mdList = &api.TestCaseMetadataList{
	Values: []*api.TestCaseMetadata{
		{
			TestCase: &api.TestCase{
				Id: &api.TestCase_Id{
					Value: "tast/test001",
				},
				Name: "tastTest",
				Tags: []*api.TestCase_Tag{
					{Value: "attr1"},
					{Value: "attr2"},
				},
			},
			TestCaseExec: &api.TestCaseExec{
				TestHarness: &api.TestHarness{
					TestHarnessType: &api.TestHarness_Tast_{
						Tast: &api.TestHarness_Tast{},
					},
				},
			},
			TestCaseInfo: &api.TestCaseInfo{
				Owners: []*api.Contact{
					{Email: "someone1@chromium.org"},
					{Email: "someone2@chromium.org"},
				},
			},
		},
		{
			TestCase: &api.TestCase{
				Id: &api.TestCase_Id{
					Value: "tauto/test002",
				},
				Name: "tautoTest",
				Tags: []*api.TestCase_Tag{
					{Value: "attr1"},
					{Value: "attr2"},
				},
			},
			TestCaseExec: &api.TestCaseExec{
				TestHarness: &api.TestHarness{
					TestHarnessType: &api.TestHarness_Tauto_{
						Tauto: &api.TestHarness_Tauto{},
					},
				},
			},
			TestCaseInfo: &api.TestCaseInfo{
				Owners: []*api.Contact{
					{Email: "someone1@chromium.org"},
					{Email: "someone2@chromium.org"},
				},
			},
		},
	},
}

// TestDriverToTestsMapping make sure driverToTestsMapping return correct values.
func TestDriverToTestsMapping(t *testing.T) {
	var buf bytes.Buffer
	logger := log.New(&buf, "logger: ", log.Lshortfile)
	expectedDriverCount := 2

	driverToTests, err := driverToTestsMapping(logger, mdList.Values)
	if err != nil {
		t.Fatal("Failed to call driverToTestsMapping: ", err)
	}

	if len(driverToTests) != expectedDriverCount {
		t.Fatalf("Got unexpected number of drivers from driverToTestsMapping %d: want %d",
			len(driverToTests), expectedDriverCount)
	}

	hasTast := false
	hasTauto := false
	for d, ts := range driverToTests {
		var expected []*api.TestCaseMetadata
		switch d.Name() {
		case "tast":
			expected = []*api.TestCaseMetadata{mdList.Values[0]}
			hasTast = true
		case "tauto":
			expected = []*api.TestCaseMetadata{mdList.Values[1]}
			hasTauto = true
		default:
			t.Fatal("Unexpected driver type returned from driverToTestsMapping: ", d.Name())
		}
		if diff := cmp.Diff(ts, expected); diff != "" {
			t.Errorf("Got unexpected data from driverToTestsMapping (-got +want):\n%s", diff)
		}
	}
	if !hasTast {
		t.Error("Did not get tast driver from driverToTestsMapping")
	}
	if !hasTauto {
		t.Error("Did not get tauto driver from driverToTestsMapping")
	}
}