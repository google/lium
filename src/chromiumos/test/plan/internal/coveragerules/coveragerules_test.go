// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package coveragerules_test

import (
	"bytes"
	"chromiumos/test/plan/internal/coveragerules"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	buildpb "go.chromium.org/chromiumos/config/go/build/api"
	testpb "go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/chromiumos/config/go/test/plan"
)

// buildMetadata is a convenience to reduce boilerplate when creating
// SystemImage_BuildMetadata in test cases.
func buildMetadata(overlay, kernelVersion, chipsetOverlay, arcVersion string) *buildpb.SystemImage_BuildMetadata {
	return &buildpb.SystemImage_BuildMetadata{
		BuildTarget: &buildpb.SystemImage_BuildTarget{
			PortageBuildTarget: &buildpb.Portage_BuildTarget{
				OverlayName: overlay,
			},
		},
		PackageSummary: &buildpb.SystemImage_BuildMetadata_PackageSummary{
			Kernel: &buildpb.SystemImage_BuildMetadata_Kernel{
				Version: kernelVersion,
			},
			Chipset: &buildpb.SystemImage_BuildMetadata_Chipset{
				Overlay: chipsetOverlay,
			},
			Arc: &buildpb.SystemImage_BuildMetadata_Arc{
				Version: arcVersion,
			},
		},
	}
}

var buildMetadataList = &buildpb.SystemImage_BuildMetadataList{
	Values: []*buildpb.SystemImage_BuildMetadata{
		buildMetadata("project1", "4.14", "chipsetA", ""),
		buildMetadata("project2", "4.14", "chipsetB", ""),
		buildMetadata("project3", "5.4", "chipsetA", ""),
		buildMetadata("project4", "3.18", "chipsetC", "R"),
		buildMetadata("project5", "4.14", "chipsetA", ""),
		buildMetadata("project6", "4.14", "chipsetB", "P"),
		buildMetadata("missingkernelversionproject", "0.0", "", ""),
	},
}
var dutAttributeList = &testpb.DutAttributeList{
	DutAttributes: []*testpb.DutAttribute{
		{
			Id:        &testpb.DutAttribute_Id{Value: "fingerprint_location"},
			FieldPath: "design_list.configs.hardware_features.fingerprint.location",
		},
		{
			Id:        &testpb.DutAttribute_Id{Value: "system_build_target"},
			FieldPath: "software_configs.system_build_target.portage_build_target.overlay_name",
		},
	},
}

func TestGenerate(t *testing.T) {
	tests := []struct {
		name     string
		input    *plan.SourceTestPlan
		expected []*testpb.CoverageRule
	}{
		{
			name: "kernel versions",
			input: &plan.SourceTestPlan{
				Requirements: &plan.SourceTestPlan_Requirements{
					KernelVersions: &plan.SourceTestPlan_Requirements_KernelVersions{},
				},
				TestTags:        []string{"kernel"},
				TestTagExcludes: []string{"flaky"},
			},
			expected: []*testpb.CoverageRule{
				{
					Name: "kernel:3.18",
					DutCriteria: []*testpb.DutCriterion{
						{
							AttributeId: &testpb.DutAttribute_Id{
								Value: "system_build_target",
							},
							Values: []string{"project4"},
						},
					},
					TestSuites: []*testpb.TestSuite{
						{
							TestCaseTagCriteria: &testpb.TestSuite_TestCaseTagCriteria{
								Tags:        []string{"kernel"},
								TagExcludes: []string{"flaky"},
							},
						},
					},
				},
				{
					Name: "kernel:4.14",
					DutCriteria: []*testpb.DutCriterion{
						{
							AttributeId: &testpb.DutAttribute_Id{
								Value: "system_build_target",
							},
							Values: []string{"project1", "project2", "project5", "project6"},
						},
					},
					TestSuites: []*testpb.TestSuite{
						{
							TestCaseTagCriteria: &testpb.TestSuite_TestCaseTagCriteria{
								Tags:        []string{"kernel"},
								TagExcludes: []string{"flaky"},
							},
						},
					},
				},
				{
					Name: "kernel:5.4",
					DutCriteria: []*testpb.DutCriterion{
						{
							AttributeId: &testpb.DutAttribute_Id{
								Value: "system_build_target",
							},
							Values: []string{"project3"},
						},
					},
					TestSuites: []*testpb.TestSuite{
						{
							TestCaseTagCriteria: &testpb.TestSuite_TestCaseTagCriteria{
								Tags:        []string{"kernel"},
								TagExcludes: []string{"flaky"},
							},
						},
					},
				},
			},
		},
		{
			name: "soc families",
			input: &plan.SourceTestPlan{
				Requirements: &plan.SourceTestPlan_Requirements{
					SocFamilies: &plan.SourceTestPlan_Requirements_SocFamilies{},
				},
				TestTagExcludes: []string{"flaky"},
			},
			expected: []*testpb.CoverageRule{
				{
					Name: "soc:chipsetA",
					DutCriteria: []*testpb.DutCriterion{
						{
							AttributeId: &testpb.DutAttribute_Id{
								Value: "system_build_target",
							},
							Values: []string{"project1", "project3", "project5"},
						},
					},
					TestSuites: []*testpb.TestSuite{
						{
							TestCaseTagCriteria: &testpb.TestSuite_TestCaseTagCriteria{
								TagExcludes: []string{"flaky"},
							},
						},
					},
				},
				{
					Name: "soc:chipsetB",
					DutCriteria: []*testpb.DutCriterion{
						{
							AttributeId: &testpb.DutAttribute_Id{
								Value: "system_build_target",
							},
							Values: []string{"project2", "project6"},
						},
					},
					TestSuites: []*testpb.TestSuite{
						{
							TestCaseTagCriteria: &testpb.TestSuite_TestCaseTagCriteria{
								TagExcludes: []string{"flaky"},
							},
						},
					},
				},
				{
					Name: "soc:chipsetC",
					DutCriteria: []*testpb.DutCriterion{
						{
							AttributeId: &testpb.DutAttribute_Id{
								Value: "system_build_target",
							},
							Values: []string{"project4"},
						},
					},
					TestSuites: []*testpb.TestSuite{
						{
							TestCaseTagCriteria: &testpb.TestSuite_TestCaseTagCriteria{
								TagExcludes: []string{"flaky"},
							},
						},
					},
				},
			},
		},
		{
			name: "build targets and designs",
			input: &plan.SourceTestPlan{
				Requirements: &plan.SourceTestPlan_Requirements{
					KernelVersions: &plan.SourceTestPlan_Requirements_KernelVersions{},
					Fingerprint:    &plan.SourceTestPlan_Requirements_Fingerprint{},
				},
				TestTags: []string{"kernel", "fingerprint"},
			},
			expected: []*testpb.CoverageRule{
				{
					Name: "fp:present",
					DutCriteria: []*testpb.DutCriterion{
						{
							AttributeId: &testpb.DutAttribute_Id{
								Value: "fingerprint_location",
							},
							Values: []string{
								"POWER_BUTTON_TOP_LEFT",
								"KEYBOARD_BOTTOM_LEFT",
								"KEYBOARD_BOTTOM_RIGHT",
								"KEYBOARD_TOP_RIGHT",
								"RIGHT_SIDE",
								"LEFT_SIDE",
								"PRESENT",
							},
						},
					},
					TestSuites: []*testpb.TestSuite{
						{
							TestCaseTagCriteria: &testpb.TestSuite_TestCaseTagCriteria{
								Tags: []string{"kernel", "fingerprint"},
							},
						},
					},
				},
				{
					Name: "kernel:3.18",
					DutCriteria: []*testpb.DutCriterion{
						{
							AttributeId: &testpb.DutAttribute_Id{
								Value: "system_build_target",
							},
							Values: []string{"project4"},
						},
					},
					TestSuites: []*testpb.TestSuite{
						{
							TestCaseTagCriteria: &testpb.TestSuite_TestCaseTagCriteria{
								Tags: []string{"kernel", "fingerprint"},
							},
						},
					},
				},
				{
					Name: "kernel:4.14",
					DutCriteria: []*testpb.DutCriterion{
						{
							AttributeId: &testpb.DutAttribute_Id{
								Value: "system_build_target",
							},
							Values: []string{"project1", "project2", "project5", "project6"},
						},
					},
					TestSuites: []*testpb.TestSuite{
						{
							TestCaseTagCriteria: &testpb.TestSuite_TestCaseTagCriteria{
								Tags: []string{"kernel", "fingerprint"},
							},
						},
					},
				},
				{
					Name: "kernel:5.4",
					DutCriteria: []*testpb.DutCriterion{
						{
							AttributeId: &testpb.DutAttribute_Id{
								Value: "system_build_target",
							},
							Values: []string{"project3"},
						},
					},
					TestSuites: []*testpb.TestSuite{
						{
							TestCaseTagCriteria: &testpb.TestSuite_TestCaseTagCriteria{
								Tags: []string{"kernel", "fingerprint"},
							},
						},
					},
				},
			},
		},
		{
			name: "multiple requirements",
			input: &plan.SourceTestPlan{
				Requirements: &plan.SourceTestPlan_Requirements{
					KernelVersions: &plan.SourceTestPlan_Requirements_KernelVersions{},
					SocFamilies:    &plan.SourceTestPlan_Requirements_SocFamilies{},
					ArcVersions:    &plan.SourceTestPlan_Requirements_ArcVersions{},
				},
				TestTags: []string{"kernel", "arc"},
			},
			expected: []*testpb.CoverageRule{
				{
					Name: "kernel:3.18_soc:chipsetC_arc:R",
					DutCriteria: []*testpb.DutCriterion{
						{
							AttributeId: &testpb.DutAttribute_Id{
								Value: "system_build_target",
							},
							Values: []string{"project4"},
						},
					},
					TestSuites: []*testpb.TestSuite{
						{
							TestCaseTagCriteria: &testpb.TestSuite_TestCaseTagCriteria{
								Tags: []string{"kernel", "arc"},
							},
						},
					},
				},
				{
					Name: "kernel:4.14_soc:chipsetA",
					DutCriteria: []*testpb.DutCriterion{
						{
							AttributeId: &testpb.DutAttribute_Id{
								Value: "system_build_target",
							},
							Values: []string{"project1", "project5"},
						},
					},
					TestSuites: []*testpb.TestSuite{
						{
							TestCaseTagCriteria: &testpb.TestSuite_TestCaseTagCriteria{
								Tags: []string{"kernel", "arc"},
							},
						},
					},
				},
				{
					Name: "kernel:4.14_soc:chipsetB_arc:P",
					DutCriteria: []*testpb.DutCriterion{
						{
							AttributeId: &testpb.DutAttribute_Id{
								Value: "system_build_target",
							},
							Values: []string{"project6"},
						},
					},
					TestSuites: []*testpb.TestSuite{
						{
							TestCaseTagCriteria: &testpb.TestSuite_TestCaseTagCriteria{
								Tags: []string{"kernel", "arc"},
							},
						},
					},
				},
				{
					Name: "kernel:5.4_soc:chipsetA",
					DutCriteria: []*testpb.DutCriterion{
						{
							AttributeId: &testpb.DutAttribute_Id{
								Value: "system_build_target",
							},
							Values: []string{"project3"},
						},
					},
					TestSuites: []*testpb.TestSuite{
						{
							TestCaseTagCriteria: &testpb.TestSuite_TestCaseTagCriteria{
								Tags: []string{"kernel", "arc"},
							},
						},
					},
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			outputs, err := coveragerules.Generate(test.input, buildMetadataList, dutAttributeList)
			if err != nil {
				t.Fatalf("coveragerules.Generate failed: %s", err)
			}
			if diff := cmp.Diff(
				test.expected,
				outputs,
				cmpopts.SortSlices(func(i, j string) bool {
					return i < j
				}),
			); diff != "" {
				t.Errorf("coveragerules.Generate returned unexpected diff (-want +got):\n%s", diff)
			}
		})
	}
}
func TestGenerateErrors(t *testing.T) {
	tests := []struct {
		name             string
		input            *plan.SourceTestPlan
		dutAttributeList *testpb.DutAttributeList
		expectedError    string
	}{
		{
			name: "no requirements",
			input: &plan.SourceTestPlan{
				EnabledTestEnvironments: []plan.SourceTestPlan_TestEnvironment{
					plan.SourceTestPlan_HARDWARE,
				},
			},
			dutAttributeList: dutAttributeList,
			expectedError:    "at least one requirement must be set in SourceTestPlan",
		},
		{
			name: "empty requirements",
			input: &plan.SourceTestPlan{
				EnabledTestEnvironments: []plan.SourceTestPlan_TestEnvironment{
					plan.SourceTestPlan_HARDWARE,
				},
				Requirements: &plan.SourceTestPlan_Requirements{},
			},
			dutAttributeList: dutAttributeList,
			expectedError:    "at least one requirement must be set in SourceTestPlan",
		},
		{
			name: "unimplemented requirement",
			input: &plan.SourceTestPlan{
				Requirements: &plan.SourceTestPlan_Requirements{
					ChromeosConfig: &plan.SourceTestPlan_Requirements_ChromeOSConfig{},
				},
			},
			dutAttributeList: dutAttributeList,
			expectedError:    `unimplemented requirement "SourceTestPlan_Requirements_ChromeOSConfig"`,
		},
		{
			name: "invalid dut attributes",
			input: &plan.SourceTestPlan{
				EnabledTestEnvironments: []plan.SourceTestPlan_TestEnvironment{
					plan.SourceTestPlan_HARDWARE,
				},
				Requirements: &plan.SourceTestPlan_Requirements{
					KernelVersions: &plan.SourceTestPlan_Requirements_KernelVersions{},
				},
			},
			dutAttributeList: &testpb.DutAttributeList{
				DutAttributes: []*testpb.DutAttribute{
					{
						Id: &testpb.DutAttribute_Id{
							Value: "miscdutattr",
						},
						FieldPath: "a.b.c",
					},
				},
			},
			expectedError: "CoverageRule contains invalid DutAttributes",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := coveragerules.Generate(
				test.input, buildMetadataList, test.dutAttributeList,
			); err == nil {
				t.Errorf("Expected error from coveragerules.Generate")
			} else if !strings.Contains(err.Error(), test.expectedError) {
				t.Errorf("Got error %q, wanted error to contain %q", err.Error(), test.expectedError)
			}
		})
	}
}

func TestWriteTextSummary(t *testing.T) {
	coverageRules := []*testpb.CoverageRule{
		{
			Name: "rule1",
			DutCriteria: []*testpb.DutCriterion{
				{
					AttributeId: &testpb.DutAttribute_Id{
						Value: "attridA",
					},
					Values: []string{"verylongdutattributevalue", "attrv2"},
				},
				{
					AttributeId: &testpb.DutAttribute_Id{
						Value: "longdutattributeid",
					},
					Values: []string{"attrv70"},
				},
			},
		},
		{
			Name: "rule2withalongname",
			DutCriteria: []*testpb.DutCriterion{
				{
					AttributeId: &testpb.DutAttribute_Id{
						Value: "attridB",
					},
					Values: []string{"attrv3"},
				},
			},
		},
	}

	var output bytes.Buffer

	expectedOutput := `
name                  attribute_id          attribute_values
rule1                 attridA               attrv2|verylongdutattributevalue
rule1                 longdutattributeid    attrv70
rule2withalongname    attridB               attrv3
`

	if err := coveragerules.WriteTextSummary(&output, coverageRules); err != nil {
		t.Fatalf("coveragerules.WriteTextSummary failed: %s", err)
	}

	if strings.TrimSpace(output.String()) != strings.TrimSpace(expectedOutput) {
		t.Errorf("coverageRules.WriteTextSummary returned %s, want %s", output.String(), expectedOutput)
	}
}
