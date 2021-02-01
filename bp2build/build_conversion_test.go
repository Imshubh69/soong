// Copyright 2020 Google Inc. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package bp2build

import (
	"android/soong/android"
	"android/soong/genrule"
	"testing"
)

func TestGenerateSoongModuleTargets(t *testing.T) {
	testCases := []struct {
		bp                  string
		expectedBazelTarget string
	}{
		{
			bp: `custom {
	name: "foo",
}
		`,
			expectedBazelTarget: `soong_module(
    name = "foo",
    soong_module_name = "foo",
    soong_module_type = "custom",
    soong_module_variant = "",
    soong_module_deps = [
    ],
)`,
		},
		{
			bp: `custom {
	name: "foo",
	ramdisk: true,
}
		`,
			expectedBazelTarget: `soong_module(
    name = "foo",
    soong_module_name = "foo",
    soong_module_type = "custom",
    soong_module_variant = "",
    soong_module_deps = [
    ],
    ramdisk = True,
)`,
		},
		{
			bp: `custom {
	name: "foo",
	owner: "a_string_with\"quotes\"_and_\\backslashes\\\\",
}
		`,
			expectedBazelTarget: `soong_module(
    name = "foo",
    soong_module_name = "foo",
    soong_module_type = "custom",
    soong_module_variant = "",
    soong_module_deps = [
    ],
    owner = "a_string_with\"quotes\"_and_\\backslashes\\\\",
)`,
		},
		{
			bp: `custom {
	name: "foo",
	required: ["bar"],
}
		`,
			expectedBazelTarget: `soong_module(
    name = "foo",
    soong_module_name = "foo",
    soong_module_type = "custom",
    soong_module_variant = "",
    soong_module_deps = [
    ],
    required = [
        "bar",
    ],
)`,
		},
		{
			bp: `custom {
	name: "foo",
	target_required: ["qux", "bazqux"],
}
		`,
			expectedBazelTarget: `soong_module(
    name = "foo",
    soong_module_name = "foo",
    soong_module_type = "custom",
    soong_module_variant = "",
    soong_module_deps = [
    ],
    target_required = [
        "qux",
        "bazqux",
    ],
)`,
		},
		{
			bp: `custom {
	name: "foo",
	dist: {
		targets: ["goal_foo"],
		tag: ".foo",
	},
	dists: [
		{
			targets: ["goal_bar"],
			tag: ".bar",
		},
	],
}
		`,
			expectedBazelTarget: `soong_module(
    name = "foo",
    soong_module_name = "foo",
    soong_module_type = "custom",
    soong_module_variant = "",
    soong_module_deps = [
    ],
    dist = {
        "tag": ".foo",
        "targets": [
            "goal_foo",
        ],
    },
    dists = [
        {
            "tag": ".bar",
            "targets": [
                "goal_bar",
            ],
        },
    ],
)`,
		},
		{
			bp: `custom {
	name: "foo",
	required: ["bar"],
	target_required: ["qux", "bazqux"],
	ramdisk: true,
	owner: "custom_owner",
	dists: [
		{
			tag: ".tag",
			targets: ["my_goal"],
		},
	],
}
		`,
			expectedBazelTarget: `soong_module(
    name = "foo",
    soong_module_name = "foo",
    soong_module_type = "custom",
    soong_module_variant = "",
    soong_module_deps = [
    ],
    dists = [
        {
            "tag": ".tag",
            "targets": [
                "my_goal",
            ],
        },
    ],
    owner = "custom_owner",
    ramdisk = True,
    required = [
        "bar",
    ],
    target_required = [
        "qux",
        "bazqux",
    ],
)`,
		},
	}

	dir := "."
	for _, testCase := range testCases {
		config := android.TestConfig(buildDir, nil, testCase.bp, nil)
		ctx := android.NewTestContext(config)
		ctx.RegisterModuleType("custom", customModuleFactory)
		ctx.Register()

		_, errs := ctx.ParseFileList(dir, []string{"Android.bp"})
		android.FailIfErrored(t, errs)
		_, errs = ctx.PrepareBuildActions(config)
		android.FailIfErrored(t, errs)

		bazelTargets := GenerateSoongModuleTargets(ctx.Context.Context, QueryView)[dir]
		if actualCount, expectedCount := len(bazelTargets), 1; actualCount != expectedCount {
			t.Fatalf("Expected %d bazel target, got %d", expectedCount, actualCount)
		}

		actualBazelTarget := bazelTargets[0]
		if actualBazelTarget.content != testCase.expectedBazelTarget {
			t.Errorf(
				"Expected generated Bazel target to be '%s', got '%s'",
				testCase.expectedBazelTarget,
				actualBazelTarget.content,
			)
		}
	}
}

func TestGenerateBazelTargetModules(t *testing.T) {
	testCases := []struct {
		bp                  string
		expectedBazelTarget string
	}{
		{
			bp: `custom {
	name: "foo",
    string_list_prop: ["a", "b"],
    string_prop: "a",
}`,
			expectedBazelTarget: `custom(
    name = "foo",
    string_list_prop = [
        "a",
        "b",
    ],
    string_prop = "a",
)`,
		},
	}

	dir := "."
	for _, testCase := range testCases {
		config := android.TestConfig(buildDir, nil, testCase.bp, nil)
		ctx := android.NewTestContext(config)
		ctx.RegisterModuleType("custom", customModuleFactory)
		ctx.RegisterBp2BuildMutator("custom", customBp2BuildMutator)
		ctx.RegisterForBazelConversion()

		_, errs := ctx.ParseFileList(dir, []string{"Android.bp"})
		android.FailIfErrored(t, errs)
		_, errs = ctx.ResolveDependencies(config)
		android.FailIfErrored(t, errs)

		bazelTargets := GenerateSoongModuleTargets(ctx.Context.Context, Bp2Build)[dir]
		if actualCount, expectedCount := len(bazelTargets), 1; actualCount != expectedCount {
			t.Fatalf("Expected %d bazel target, got %d", expectedCount, actualCount)
		}

		actualBazelTarget := bazelTargets[0]
		if actualBazelTarget.content != testCase.expectedBazelTarget {
			t.Errorf(
				"Expected generated Bazel target to be '%s', got '%s'",
				testCase.expectedBazelTarget,
				actualBazelTarget.content,
			)
		}
	}
}

func TestLoadStatements(t *testing.T) {
	testCases := []struct {
		bazelTargets           BazelTargets
		expectedLoadStatements string
	}{
		{
			bazelTargets: BazelTargets{
				BazelTarget{
					name:            "foo",
					ruleClass:       "cc_library",
					bzlLoadLocation: "//build/bazel/rules:cc.bzl",
				},
			},
			expectedLoadStatements: `load("//build/bazel/rules:cc.bzl", "cc_library")`,
		},
		{
			bazelTargets: BazelTargets{
				BazelTarget{
					name:            "foo",
					ruleClass:       "cc_library",
					bzlLoadLocation: "//build/bazel/rules:cc.bzl",
				},
				BazelTarget{
					name:            "bar",
					ruleClass:       "cc_library",
					bzlLoadLocation: "//build/bazel/rules:cc.bzl",
				},
			},
			expectedLoadStatements: `load("//build/bazel/rules:cc.bzl", "cc_library")`,
		},
		{
			bazelTargets: BazelTargets{
				BazelTarget{
					name:            "foo",
					ruleClass:       "cc_library",
					bzlLoadLocation: "//build/bazel/rules:cc.bzl",
				},
				BazelTarget{
					name:            "bar",
					ruleClass:       "cc_binary",
					bzlLoadLocation: "//build/bazel/rules:cc.bzl",
				},
			},
			expectedLoadStatements: `load("//build/bazel/rules:cc.bzl", "cc_binary", "cc_library")`,
		},
		{
			bazelTargets: BazelTargets{
				BazelTarget{
					name:            "foo",
					ruleClass:       "cc_library",
					bzlLoadLocation: "//build/bazel/rules:cc.bzl",
				},
				BazelTarget{
					name:            "bar",
					ruleClass:       "cc_binary",
					bzlLoadLocation: "//build/bazel/rules:cc.bzl",
				},
				BazelTarget{
					name:            "baz",
					ruleClass:       "java_binary",
					bzlLoadLocation: "//build/bazel/rules:java.bzl",
				},
			},
			expectedLoadStatements: `load("//build/bazel/rules:cc.bzl", "cc_binary", "cc_library")
load("//build/bazel/rules:java.bzl", "java_binary")`,
		},
		{
			bazelTargets: BazelTargets{
				BazelTarget{
					name:            "foo",
					ruleClass:       "cc_binary",
					bzlLoadLocation: "//build/bazel/rules:cc.bzl",
				},
				BazelTarget{
					name:            "bar",
					ruleClass:       "java_binary",
					bzlLoadLocation: "//build/bazel/rules:java.bzl",
				},
				BazelTarget{
					name:      "baz",
					ruleClass: "genrule",
					// Note: no bzlLoadLocation for native rules
				},
			},
			expectedLoadStatements: `load("//build/bazel/rules:cc.bzl", "cc_binary")
load("//build/bazel/rules:java.bzl", "java_binary")`,
		},
	}

	for _, testCase := range testCases {
		actual := testCase.bazelTargets.LoadStatements()
		expected := testCase.expectedLoadStatements
		if actual != expected {
			t.Fatalf("Expected load statements to be %s, got %s", expected, actual)
		}
	}

}

func TestGenerateBazelTargetModules_OneToMany_LoadedFromStarlark(t *testing.T) {
	testCases := []struct {
		bp                       string
		expectedBazelTarget      string
		expectedBazelTargetCount int
		expectedLoadStatements   string
	}{
		{
			bp: `custom {
    name: "bar",
}`,
			expectedBazelTarget: `my_library(
    name = "bar",
)

my_proto_library(
    name = "bar_my_proto_library_deps",
)

proto_library(
    name = "bar_proto_library_deps",
)`,
			expectedBazelTargetCount: 3,
			expectedLoadStatements: `load("//build/bazel/rules:proto.bzl", "my_proto_library", "proto_library")
load("//build/bazel/rules:rules.bzl", "my_library")`,
		},
	}

	dir := "."
	for _, testCase := range testCases {
		config := android.TestConfig(buildDir, nil, testCase.bp, nil)
		ctx := android.NewTestContext(config)
		ctx.RegisterModuleType("custom", customModuleFactory)
		ctx.RegisterBp2BuildMutator("custom_starlark", customBp2BuildMutatorFromStarlark)
		ctx.RegisterForBazelConversion()

		_, errs := ctx.ParseFileList(dir, []string{"Android.bp"})
		android.FailIfErrored(t, errs)
		_, errs = ctx.ResolveDependencies(config)
		android.FailIfErrored(t, errs)

		bazelTargets := GenerateSoongModuleTargets(ctx.Context.Context, Bp2Build)[dir]
		if actualCount := len(bazelTargets); actualCount != testCase.expectedBazelTargetCount {
			t.Fatalf("Expected %d bazel target, got %d", testCase.expectedBazelTargetCount, actualCount)
		}

		actualBazelTargets := bazelTargets.String()
		if actualBazelTargets != testCase.expectedBazelTarget {
			t.Errorf(
				"Expected generated Bazel target to be '%s', got '%s'",
				testCase.expectedBazelTarget,
				actualBazelTargets,
			)
		}

		actualLoadStatements := bazelTargets.LoadStatements()
		if actualLoadStatements != testCase.expectedLoadStatements {
			t.Errorf(
				"Expected generated load statements to be '%s', got '%s'",
				testCase.expectedLoadStatements,
				actualLoadStatements,
			)
		}
	}
}

func TestModuleTypeBp2Build(t *testing.T) {
	testCases := []struct {
		moduleTypeUnderTest                string
		moduleTypeUnderTestFactory         android.ModuleFactory
		moduleTypeUnderTestBp2BuildMutator func(android.TopDownMutatorContext)
		bp                                 string
		expectedBazelTarget                string
		description                        string
	}{
		{
			description:                        "filegroup with no srcs",
			moduleTypeUnderTest:                "filegroup",
			moduleTypeUnderTestFactory:         android.FileGroupFactory,
			moduleTypeUnderTestBp2BuildMutator: android.FilegroupBp2Build,
			bp: `filegroup {
	name: "foo",
	srcs: [],
}`,
			expectedBazelTarget: `filegroup(
    name = "foo",
    srcs = [
    ],
)`,
		},
		{
			description:                        "filegroup with srcs",
			moduleTypeUnderTest:                "filegroup",
			moduleTypeUnderTestFactory:         android.FileGroupFactory,
			moduleTypeUnderTestBp2BuildMutator: android.FilegroupBp2Build,
			bp: `filegroup {
	name: "foo",
	srcs: ["a", "b"],
}`,
			expectedBazelTarget: `filegroup(
    name = "foo",
    srcs = [
        "a",
        "b",
    ],
)`,
		},
		{
			description:                        "genrule with command line variable replacements",
			moduleTypeUnderTest:                "genrule",
			moduleTypeUnderTestFactory:         genrule.GenRuleFactory,
			moduleTypeUnderTestBp2BuildMutator: genrule.GenruleBp2Build,
			bp: `genrule {
    name: "foo",
    out: ["foo.out"],
    srcs: ["foo.in"],
    tools: [":foo.tool"],
    cmd: "$(location :foo.tool) --genDir=$(genDir) arg $(in) $(out)",
}`,
			expectedBazelTarget: `genrule(
    name = "foo",
    cmd = "$(location :foo.tool) --genDir=$(GENDIR) arg $(SRCS) $(OUTS)",
    outs = [
        "foo.out",
    ],
    srcs = [
        "foo.in",
    ],
    tools = [
        ":foo.tool",
    ],
)`,
		},
		{
			description:                        "genrule using $(locations :label)",
			moduleTypeUnderTest:                "genrule",
			moduleTypeUnderTestFactory:         genrule.GenRuleFactory,
			moduleTypeUnderTestBp2BuildMutator: genrule.GenruleBp2Build,
			bp: `genrule {
    name: "foo",
    out: ["foo.out"],
    srcs: ["foo.in"],
    tools: [":foo.tools"],
    cmd: "$(locations :foo.tools) -s $(out) $(in)",
}`,
			expectedBazelTarget: `genrule(
    name = "foo",
    cmd = "$(locations :foo.tools) -s $(OUTS) $(SRCS)",
    outs = [
        "foo.out",
    ],
    srcs = [
        "foo.in",
    ],
    tools = [
        ":foo.tools",
    ],
)`,
		},
		{
			description:                        "genrule using $(location) label should substitute first tool label automatically",
			moduleTypeUnderTest:                "genrule",
			moduleTypeUnderTestFactory:         genrule.GenRuleFactory,
			moduleTypeUnderTestBp2BuildMutator: genrule.GenruleBp2Build,
			bp: `genrule {
    name: "foo",
    out: ["foo.out"],
    srcs: ["foo.in"],
    tool_files: [":foo.tool", ":other.tool"],
    cmd: "$(location) -s $(out) $(in)",
}`,
			expectedBazelTarget: `genrule(
    name = "foo",
    cmd = "$(location :foo.tool) -s $(OUTS) $(SRCS)",
    outs = [
        "foo.out",
    ],
    srcs = [
        "foo.in",
    ],
    tools = [
        ":foo.tool",
        ":other.tool",
    ],
)`,
		},
		{
			description:                        "genrule using $(locations) label should substitute first tool label automatically",
			moduleTypeUnderTest:                "genrule",
			moduleTypeUnderTestFactory:         genrule.GenRuleFactory,
			moduleTypeUnderTestBp2BuildMutator: genrule.GenruleBp2Build,
			bp: `genrule {
    name: "foo",
    out: ["foo.out"],
    srcs: ["foo.in"],
    tools: [":foo.tool", ":other.tool"],
    cmd: "$(locations) -s $(out) $(in)",
}`,
			expectedBazelTarget: `genrule(
    name = "foo",
    cmd = "$(locations :foo.tool) -s $(OUTS) $(SRCS)",
    outs = [
        "foo.out",
    ],
    srcs = [
        "foo.in",
    ],
    tools = [
        ":foo.tool",
        ":other.tool",
    ],
)`,
		},
		{
			description:                        "genrule without tools or tool_files can convert successfully",
			moduleTypeUnderTest:                "genrule",
			moduleTypeUnderTestFactory:         genrule.GenRuleFactory,
			moduleTypeUnderTestBp2BuildMutator: genrule.GenruleBp2Build,
			bp: `genrule {
    name: "foo",
    out: ["foo.out"],
    srcs: ["foo.in"],
    cmd: "cp $(in) $(out)",
}`,
			expectedBazelTarget: `genrule(
    name = "foo",
    cmd = "cp $(SRCS) $(OUTS)",
    outs = [
        "foo.out",
    ],
    srcs = [
        "foo.in",
    ],
)`,
		},
	}

	dir := "."
	for _, testCase := range testCases {
		config := android.TestConfig(buildDir, nil, testCase.bp, nil)
		ctx := android.NewTestContext(config)
		ctx.RegisterModuleType(testCase.moduleTypeUnderTest, testCase.moduleTypeUnderTestFactory)
		ctx.RegisterBp2BuildMutator(testCase.moduleTypeUnderTest, testCase.moduleTypeUnderTestBp2BuildMutator)
		ctx.RegisterForBazelConversion()

		_, errs := ctx.ParseFileList(dir, []string{"Android.bp"})
		android.FailIfErrored(t, errs)
		_, errs = ctx.ResolveDependencies(config)
		android.FailIfErrored(t, errs)

		bazelTargets := GenerateSoongModuleTargets(ctx.Context.Context, Bp2Build)[dir]
		if actualCount, expectedCount := len(bazelTargets), 1; actualCount != expectedCount {
			t.Fatalf("%s: Expected %d bazel target, got %d", testCase.description, expectedCount, actualCount)
		}

		actualBazelTarget := bazelTargets[0]
		if actualBazelTarget.content != testCase.expectedBazelTarget {
			t.Errorf(
				"%s: Expected generated Bazel target to be '%s', got '%s'",
				testCase.description,
				testCase.expectedBazelTarget,
				actualBazelTarget.content,
			)
		}
	}
}
