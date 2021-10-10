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
	"fmt"
	"strings"
	"testing"

	"android/soong/android"
	"android/soong/python"
)

func TestGenerateSoongModuleTargets(t *testing.T) {
	testCases := []struct {
		description         string
		bp                  string
		expectedBazelTarget string
	}{
		{
			description: "only name",
			bp: `custom { name: "foo" }
    `,
			expectedBazelTarget: `soong_module(
    name = "foo",
    soong_module_name = "foo",
    soong_module_type = "custom",
    soong_module_variant = "",
    soong_module_deps = [
    ],
    bool_prop = False,
)`,
		},
		{
			description: "handles bool",
			bp: `custom {
  name: "foo",
  bool_prop: true,
}
    `,
			expectedBazelTarget: `soong_module(
    name = "foo",
    soong_module_name = "foo",
    soong_module_type = "custom",
    soong_module_variant = "",
    soong_module_deps = [
    ],
    bool_prop = True,
)`,
		},
		{
			description: "string escaping",
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
    bool_prop = False,
    owner = "a_string_with\"quotes\"_and_\\backslashes\\\\",
)`,
		},
		{
			description: "single item string list",
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
    bool_prop = False,
    required = ["bar"],
)`,
		},
		{
			description: "list of strings",
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
    bool_prop = False,
    target_required = [
        "qux",
        "bazqux",
    ],
)`,
		},
		{
			description: "dist/dists",
			bp: `custom {
  name: "foo",
  dist: {
    targets: ["goal_foo"],
    tag: ".foo",
  },
  dists: [{
    targets: ["goal_bar"],
    tag: ".bar",
  }],
}
    `,
			expectedBazelTarget: `soong_module(
    name = "foo",
    soong_module_name = "foo",
    soong_module_type = "custom",
    soong_module_variant = "",
    soong_module_deps = [
    ],
    bool_prop = False,
    dist = {
        "tag": ".foo",
        "targets": ["goal_foo"],
    },
    dists = [{
        "tag": ".bar",
        "targets": ["goal_bar"],
    }],
)`,
		},
		{
			description: "put it together",
			bp: `custom {
  name: "foo",
  required: ["bar"],
  target_required: ["qux", "bazqux"],
  bool_prop: true,
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
    bool_prop = True,
    dists = [{
        "tag": ".tag",
        "targets": ["my_goal"],
    }],
    owner = "custom_owner",
    required = ["bar"],
    target_required = [
        "qux",
        "bazqux",
    ],
)`,
		},
	}

	dir := "."
	for _, testCase := range testCases {
		t.Run(testCase.description, func(t *testing.T) {
			config := android.TestConfig(buildDir, nil, testCase.bp, nil)
			ctx := android.NewTestContext(config)

			ctx.RegisterModuleType("custom", customModuleFactory)
			ctx.Register()

			_, errs := ctx.ParseFileList(dir, []string{"Android.bp"})
			android.FailIfErrored(t, errs)
			_, errs = ctx.PrepareBuildActions(config)
			android.FailIfErrored(t, errs)

			codegenCtx := NewCodegenContext(config, *ctx.Context, QueryView)
			bazelTargets, err := generateBazelTargetsForDir(codegenCtx, dir)
			android.FailIfErrored(t, err)
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
		})
	}
}

func TestGenerateBazelTargetModules(t *testing.T) {
	testCases := []bp2buildTestCase{
		{
			blueprint: `custom {
	name: "foo",
    string_list_prop: ["a", "b"],
    string_prop: "a",
    bazel_module: { bp2build_available: true },
}`,
			expectedBazelTargets: []string{`custom(
    name = "foo",
    string_list_prop = [
        "a",
        "b",
    ],
    string_prop = "a",
)`,
			},
		},
		{
			blueprint: `custom {
	name: "control_characters",
    string_list_prop: ["\t", "\n"],
    string_prop: "a\t\n\r",
    bazel_module: { bp2build_available: true },
}`,
			expectedBazelTargets: []string{`custom(
    name = "control_characters",
    string_list_prop = [
        "\t",
        "\n",
    ],
    string_prop = "a\t\n\r",
)`,
			},
		},
		{
			blueprint: `custom {
  name: "has_dep",
  arch_paths: [":dep"],
  bazel_module: { bp2build_available: true },
}

custom {
  name: "dep",
  arch_paths: ["abc"],
  bazel_module: { bp2build_available: true },
}`,
			expectedBazelTargets: []string{`custom(
    name = "dep",
    arch_paths = ["abc"],
)`,
				`custom(
    name = "has_dep",
    arch_paths = [":dep"],
)`,
			},
		},
		{
			blueprint: `custom {
    name: "arch_paths",
    arch: {
      x86: {
        arch_paths: ["abc"],
      },
    },
    bazel_module: { bp2build_available: true },
}`,
			expectedBazelTargets: []string{`custom(
    name = "arch_paths",
    arch_paths = select({
        "//build/bazel/platforms/arch:x86": ["abc"],
        "//conditions:default": [],
    }),
)`,
			},
		},
		{
			blueprint: `custom {
  name: "has_dep",
  arch: {
    x86: {
      arch_paths: [":dep"],
    },
  },
  bazel_module: { bp2build_available: true },
}

custom {
    name: "dep",
    arch_paths: ["abc"],
    bazel_module: { bp2build_available: true },
}`,
			expectedBazelTargets: []string{`custom(
    name = "dep",
    arch_paths = ["abc"],
)`,
				`custom(
    name = "has_dep",
    arch_paths = select({
        "//build/bazel/platforms/arch:x86": [":dep"],
        "//conditions:default": [],
    }),
)`,
			},
		},
		{
			blueprint: `custom {
    name: "embedded_props",
    embedded_prop: "abc",
    bazel_module: { bp2build_available: true },
}`,
			expectedBazelTargets: []string{`custom(
    name = "embedded_props",
    embedded_attr = "abc",
)`,
			},
		},
		{
			blueprint: `custom {
    name: "ptr_to_embedded_props",
    other_embedded_prop: "abc",
    bazel_module: { bp2build_available: true },
}`,
			expectedBazelTargets: []string{`custom(
    name = "ptr_to_embedded_props",
    other_embedded_attr = "abc",
)`,
			},
		},
	}

	dir := "."
	for _, testCase := range testCases {
		config := android.TestConfig(buildDir, nil, testCase.blueprint, nil)
		ctx := android.NewTestContext(config)

		registerCustomModuleForBp2buildConversion(ctx)

		_, errs := ctx.ParseFileList(dir, []string{"Android.bp"})
		if errored(t, testCase, errs) {
			continue
		}
		_, errs = ctx.ResolveDependencies(config)
		if errored(t, testCase, errs) {
			continue
		}

		codegenCtx := NewCodegenContext(config, *ctx.Context, Bp2Build)
		bazelTargets, err := generateBazelTargetsForDir(codegenCtx, dir)
		android.FailIfErrored(t, err)

		if actualCount, expectedCount := len(bazelTargets), len(testCase.expectedBazelTargets); actualCount != expectedCount {
			t.Errorf("Expected %d bazel target, got %d", expectedCount, actualCount)
		} else {
			for i, expectedBazelTarget := range testCase.expectedBazelTargets {
				actualBazelTarget := bazelTargets[i]
				if actualBazelTarget.content != expectedBazelTarget {
					t.Errorf(
						"Expected generated Bazel target to be '%s', got '%s'",
						expectedBazelTarget,
						actualBazelTarget.content,
					)
				}
			}
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
    bazel_module: { bp2build_available: true  },
}`,
			expectedBazelTarget: `my_library(
    name = "bar",
)

proto_library(
    name = "bar_proto_library_deps",
)

my_proto_library(
    name = "bar_my_proto_library_deps",
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
		ctx.RegisterBp2BuildMutator("custom", customBp2BuildMutatorFromStarlark)
		ctx.RegisterForBazelConversion()

		_, errs := ctx.ParseFileList(dir, []string{"Android.bp"})
		android.FailIfErrored(t, errs)
		_, errs = ctx.ResolveDependencies(config)
		android.FailIfErrored(t, errs)

		codegenCtx := NewCodegenContext(config, *ctx.Context, Bp2Build)
		bazelTargets, err := generateBazelTargetsForDir(codegenCtx, dir)
		android.FailIfErrored(t, err)
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
	testCases := []bp2buildTestCase{
		{
			description:                        "filegroup with does not specify srcs",
			moduleTypeUnderTest:                "filegroup",
			moduleTypeUnderTestFactory:         android.FileGroupFactory,
			moduleTypeUnderTestBp2BuildMutator: android.FilegroupBp2Build,
			blueprint: `filegroup {
    name: "fg_foo",
    bazel_module: { bp2build_available: true },
}`,
			expectedBazelTargets: []string{
				`filegroup(
    name = "fg_foo",
)`,
			},
		},
		{
			description:                        "filegroup with no srcs",
			moduleTypeUnderTest:                "filegroup",
			moduleTypeUnderTestFactory:         android.FileGroupFactory,
			moduleTypeUnderTestBp2BuildMutator: android.FilegroupBp2Build,
			blueprint: `filegroup {
    name: "fg_foo",
    srcs: [],
    bazel_module: { bp2build_available: true },
}`,
			expectedBazelTargets: []string{
				`filegroup(
    name = "fg_foo",
)`,
			},
		},
		{
			description:                        "filegroup with srcs",
			moduleTypeUnderTest:                "filegroup",
			moduleTypeUnderTestFactory:         android.FileGroupFactory,
			moduleTypeUnderTestBp2BuildMutator: android.FilegroupBp2Build,
			blueprint: `filegroup {
    name: "fg_foo",
    srcs: ["a", "b"],
    bazel_module: { bp2build_available: true },
}`,
			expectedBazelTargets: []string{`filegroup(
    name = "fg_foo",
    srcs = [
        "a",
        "b",
    ],
)`,
			},
		},
		{
			description:                        "filegroup with excludes srcs",
			moduleTypeUnderTest:                "filegroup",
			moduleTypeUnderTestFactory:         android.FileGroupFactory,
			moduleTypeUnderTestBp2BuildMutator: android.FilegroupBp2Build,
			blueprint: `filegroup {
    name: "fg_foo",
    srcs: ["a", "b"],
    exclude_srcs: ["a"],
    bazel_module: { bp2build_available: true },
}`,
			expectedBazelTargets: []string{`filegroup(
    name = "fg_foo",
    srcs = ["b"],
)`,
			},
		},
		{
			description:                        "filegroup with glob",
			moduleTypeUnderTest:                "filegroup",
			moduleTypeUnderTestFactory:         android.FileGroupFactory,
			moduleTypeUnderTestBp2BuildMutator: android.FilegroupBp2Build,
			blueprint: `filegroup {
    name: "foo",
    srcs: ["**/*.txt"],
    bazel_module: { bp2build_available: true },
}`,
			expectedBazelTargets: []string{`filegroup(
    name = "foo",
    srcs = [
        "other/a.txt",
        "other/b.txt",
        "other/subdir/a.txt",
    ],
)`,
			},
			filesystem: map[string]string{
				"other/a.txt":        "",
				"other/b.txt":        "",
				"other/subdir/a.txt": "",
				"other/file":         "",
			},
		},
		{
			description:                        "filegroup with glob in subdir",
			moduleTypeUnderTest:                "filegroup",
			moduleTypeUnderTestFactory:         android.FileGroupFactory,
			moduleTypeUnderTestBp2BuildMutator: android.FilegroupBp2Build,
			blueprint: `filegroup {
    name: "foo",
    srcs: ["a.txt"],
    bazel_module: { bp2build_available: true },
}`,
			dir: "other",
			expectedBazelTargets: []string{`filegroup(
    name = "fg_foo",
    srcs = [
        "a.txt",
        "b.txt",
        "subdir/a.txt",
    ],
)`,
			},
			filesystem: map[string]string{
				"other/Android.bp": `filegroup {
    name: "fg_foo",
    srcs: ["**/*.txt"],
    bazel_module: { bp2build_available: true },
}`,
				"other/a.txt":        "",
				"other/b.txt":        "",
				"other/subdir/a.txt": "",
				"other/file":         "",
			},
		},
		{
			description:                        "depends_on_other_dir_module",
			moduleTypeUnderTest:                "filegroup",
			moduleTypeUnderTestFactory:         android.FileGroupFactory,
			moduleTypeUnderTestBp2BuildMutator: android.FilegroupBp2Build,
			blueprint: `filegroup {
    name: "foobar",
    srcs: [
        ":foo",
        "c",
    ],
    bazel_module: { bp2build_available: true },
}`,
			expectedBazelTargets: []string{`filegroup(
    name = "foobar",
    srcs = [
        "//other:foo",
        "c",
    ],
)`,
			},
			filesystem: map[string]string{
				"other/Android.bp": `filegroup {
    name: "foo",
    srcs: ["a", "b"],
    bazel_module: { bp2build_available: true },
}`,
			},
		},
		{
			description:                        "depends_on_other_unconverted_module_error",
			moduleTypeUnderTest:                "filegroup",
			moduleTypeUnderTestFactory:         android.FileGroupFactory,
			moduleTypeUnderTestBp2BuildMutator: android.FilegroupBp2Build,
			unconvertedDepsMode:                errorModulesUnconvertedDeps,
			blueprint: `filegroup {
    name: "foobar",
    srcs: [
        ":foo",
        "c",
    ],
    bazel_module: { bp2build_available: true },
}`,
			expectedErr: fmt.Errorf(`"foobar" depends on unconverted modules: foo`),
			filesystem: map[string]string{
				"other/Android.bp": `filegroup {
    name: "foo",
    srcs: ["a", "b"],
}`,
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.description, func(t *testing.T) {
			runBp2BuildTestCase(t, func(ctx android.RegistrationContext) {}, testCase)
		})
	}
}

type bp2buildMutator = func(android.TopDownMutatorContext)

func TestAllowlistingBp2buildTargetsExplicitly(t *testing.T) {
	testCases := []struct {
		moduleTypeUnderTest                string
		moduleTypeUnderTestFactory         android.ModuleFactory
		moduleTypeUnderTestBp2BuildMutator bp2buildMutator
		bp                                 string
		expectedCount                      int
		description                        string
	}{
		{
			description:                        "explicitly unavailable",
			moduleTypeUnderTest:                "filegroup",
			moduleTypeUnderTestFactory:         android.FileGroupFactory,
			moduleTypeUnderTestBp2BuildMutator: android.FilegroupBp2Build,
			bp: `filegroup {
    name: "foo",
    srcs: ["a", "b"],
    bazel_module: { bp2build_available: false },
}`,
			expectedCount: 0,
		},
		{
			description:                        "implicitly unavailable",
			moduleTypeUnderTest:                "filegroup",
			moduleTypeUnderTestFactory:         android.FileGroupFactory,
			moduleTypeUnderTestBp2BuildMutator: android.FilegroupBp2Build,
			bp: `filegroup {
    name: "foo",
    srcs: ["a", "b"],
}`,
			expectedCount: 0,
		},
		{
			description:                        "explicitly available",
			moduleTypeUnderTest:                "filegroup",
			moduleTypeUnderTestFactory:         android.FileGroupFactory,
			moduleTypeUnderTestBp2BuildMutator: android.FilegroupBp2Build,
			bp: `filegroup {
    name: "foo",
    srcs: ["a", "b"],
    bazel_module: { bp2build_available: true },
}`,
			expectedCount: 1,
		},
		{
			description:                        "generates more than 1 target if needed",
			moduleTypeUnderTest:                "custom",
			moduleTypeUnderTestFactory:         customModuleFactory,
			moduleTypeUnderTestBp2BuildMutator: customBp2BuildMutatorFromStarlark,
			bp: `custom {
    name: "foo",
    bazel_module: { bp2build_available: true },
}`,
			expectedCount: 3,
		},
	}

	dir := "."
	for _, testCase := range testCases {
		t.Run(testCase.description, func(t *testing.T) {
			config := android.TestConfig(buildDir, nil, testCase.bp, nil)
			ctx := android.NewTestContext(config)
			ctx.RegisterModuleType(testCase.moduleTypeUnderTest, testCase.moduleTypeUnderTestFactory)
			ctx.RegisterBp2BuildMutator(testCase.moduleTypeUnderTest, testCase.moduleTypeUnderTestBp2BuildMutator)
			ctx.RegisterForBazelConversion()

			_, errs := ctx.ParseFileList(dir, []string{"Android.bp"})
			android.FailIfErrored(t, errs)
			_, errs = ctx.ResolveDependencies(config)
			android.FailIfErrored(t, errs)

			codegenCtx := NewCodegenContext(config, *ctx.Context, Bp2Build)
			bazelTargets, err := generateBazelTargetsForDir(codegenCtx, dir)
			android.FailIfErrored(t, err)
			if actualCount := len(bazelTargets); actualCount != testCase.expectedCount {
				t.Fatalf("%s: Expected %d bazel target, got %d", testCase.description, testCase.expectedCount, actualCount)
			}
		})
	}
}

func TestAllowlistingBp2buildTargetsWithConfig(t *testing.T) {
	testCases := []struct {
		moduleTypeUnderTest                string
		moduleTypeUnderTestFactory         android.ModuleFactory
		moduleTypeUnderTestBp2BuildMutator bp2buildMutator
		expectedCount                      map[string]int
		description                        string
		bp2buildConfig                     android.Bp2BuildConfig
		checkDir                           string
		fs                                 map[string]string
	}{
		{
			description:                        "test bp2build config package and subpackages config",
			moduleTypeUnderTest:                "filegroup",
			moduleTypeUnderTestFactory:         android.FileGroupFactory,
			moduleTypeUnderTestBp2BuildMutator: android.FilegroupBp2Build,
			expectedCount: map[string]int{
				"migrated":                           1,
				"migrated/but_not_really":            0,
				"migrated/but_not_really/but_really": 1,
				"not_migrated":                       0,
				"also_not_migrated":                  0,
			},
			bp2buildConfig: android.Bp2BuildConfig{
				"migrated":                android.Bp2BuildDefaultTrueRecursively,
				"migrated/but_not_really": android.Bp2BuildDefaultFalse,
				"not_migrated":            android.Bp2BuildDefaultFalse,
			},
			fs: map[string]string{
				"migrated/Android.bp":                           `filegroup { name: "a" }`,
				"migrated/but_not_really/Android.bp":            `filegroup { name: "b" }`,
				"migrated/but_not_really/but_really/Android.bp": `filegroup { name: "c" }`,
				"not_migrated/Android.bp":                       `filegroup { name: "d" }`,
				"also_not_migrated/Android.bp":                  `filegroup { name: "e" }`,
			},
		},
		{
			description:                        "test bp2build config opt-in and opt-out",
			moduleTypeUnderTest:                "filegroup",
			moduleTypeUnderTestFactory:         android.FileGroupFactory,
			moduleTypeUnderTestBp2BuildMutator: android.FilegroupBp2Build,
			expectedCount: map[string]int{
				"package-opt-in":             2,
				"package-opt-in/subpackage":  0,
				"package-opt-out":            1,
				"package-opt-out/subpackage": 0,
			},
			bp2buildConfig: android.Bp2BuildConfig{
				"package-opt-in":  android.Bp2BuildDefaultFalse,
				"package-opt-out": android.Bp2BuildDefaultTrueRecursively,
			},
			fs: map[string]string{
				"package-opt-in/Android.bp": `
filegroup { name: "opt-in-a" }
filegroup { name: "opt-in-b", bazel_module: { bp2build_available: true } }
filegroup { name: "opt-in-c", bazel_module: { bp2build_available: true } }
`,

				"package-opt-in/subpackage/Android.bp": `
filegroup { name: "opt-in-d" } // parent package not configured to DefaultTrueRecursively
`,

				"package-opt-out/Android.bp": `
filegroup { name: "opt-out-a" }
filegroup { name: "opt-out-b", bazel_module: { bp2build_available: false } }
filegroup { name: "opt-out-c", bazel_module: { bp2build_available: false } }
`,

				"package-opt-out/subpackage/Android.bp": `
filegroup { name: "opt-out-g", bazel_module: { bp2build_available: false } }
filegroup { name: "opt-out-h", bazel_module: { bp2build_available: false } }
`,
			},
		},
	}

	dir := "."
	for _, testCase := range testCases {
		fs := make(map[string][]byte)
		toParse := []string{
			"Android.bp",
		}
		for f, content := range testCase.fs {
			if strings.HasSuffix(f, "Android.bp") {
				toParse = append(toParse, f)
			}
			fs[f] = []byte(content)
		}
		config := android.TestConfig(buildDir, nil, "", fs)
		ctx := android.NewTestContext(config)
		ctx.RegisterModuleType(testCase.moduleTypeUnderTest, testCase.moduleTypeUnderTestFactory)
		ctx.RegisterBp2BuildMutator(testCase.moduleTypeUnderTest, testCase.moduleTypeUnderTestBp2BuildMutator)
		ctx.RegisterBp2BuildConfig(testCase.bp2buildConfig)
		ctx.RegisterForBazelConversion()

		_, errs := ctx.ParseFileList(dir, toParse)
		android.FailIfErrored(t, errs)
		_, errs = ctx.ResolveDependencies(config)
		android.FailIfErrored(t, errs)

		codegenCtx := NewCodegenContext(config, *ctx.Context, Bp2Build)

		// For each directory, test that the expected number of generated targets is correct.
		for dir, expectedCount := range testCase.expectedCount {
			bazelTargets, err := generateBazelTargetsForDir(codegenCtx, dir)
			android.FailIfErrored(t, err)
			if actualCount := len(bazelTargets); actualCount != expectedCount {
				t.Fatalf(
					"%s: Expected %d bazel target for %s package, got %d",
					testCase.description,
					expectedCount,
					dir,
					actualCount)
			}

		}
	}
}

func TestCombineBuildFilesBp2buildTargets(t *testing.T) {
	testCases := []bp2buildTestCase{
		{
			description:                        "filegroup bazel_module.label",
			moduleTypeUnderTest:                "filegroup",
			moduleTypeUnderTestFactory:         android.FileGroupFactory,
			moduleTypeUnderTestBp2BuildMutator: android.FilegroupBp2Build,
			blueprint: `filegroup {
    name: "fg_foo",
    bazel_module: { label: "//other:fg_foo" },
}`,
			expectedBazelTargets: []string{
				`// BUILD file`,
			},
			filesystem: map[string]string{
				"other/BUILD.bazel": `// BUILD file`,
			},
		},
		{
			description:                        "multiple bazel_module.label same BUILD",
			moduleTypeUnderTest:                "filegroup",
			moduleTypeUnderTestFactory:         android.FileGroupFactory,
			moduleTypeUnderTestBp2BuildMutator: android.FilegroupBp2Build,
			blueprint: `filegroup {
		    name: "fg_foo",
		    bazel_module: { label: "//other:fg_foo" },
		}

		filegroup {
		    name: "foo",
		    bazel_module: { label: "//other:foo" },
		}`,
			expectedBazelTargets: []string{
				`// BUILD file`,
			},
			filesystem: map[string]string{
				"other/BUILD.bazel": `// BUILD file`,
			},
		},
		{
			description:                        "filegroup bazel_module.label and bp2build in subdir",
			moduleTypeUnderTest:                "filegroup",
			moduleTypeUnderTestFactory:         android.FileGroupFactory,
			moduleTypeUnderTestBp2BuildMutator: android.FilegroupBp2Build,
			dir:                                "other",
			blueprint:                          ``,
			filesystem: map[string]string{
				"other/Android.bp": `filegroup {
				name: "fg_foo",
				bazel_module: {
					bp2build_available: true,
				},
			}
			filegroup {
				name: "fg_bar",
				bazel_module: {
					label: "//other:fg_bar"
				},
			}`,
				"other/BUILD.bazel": `// definition for fg_bar`,
			},
			expectedBazelTargets: []string{
				`filegroup(
    name = "fg_foo",
)`, `// definition for fg_bar`,
			},
		},
		{
			description:                        "filegroup bazel_module.label and filegroup bp2build",
			moduleTypeUnderTest:                "filegroup",
			moduleTypeUnderTestFactory:         android.FileGroupFactory,
			moduleTypeUnderTestBp2BuildMutator: android.FilegroupBp2Build,
			blueprint: `filegroup {
		    name: "fg_foo",
		    bazel_module: {
		      label: "//other:fg_foo",
		    },
		}

		filegroup {
		    name: "fg_bar",
		    bazel_module: {
		      bp2build_available: true,
		    },
		}`,
			expectedBazelTargets: []string{
				`filegroup(
    name = "fg_bar",
)`,
				`// BUILD file`,
			},
			filesystem: map[string]string{
				"other/BUILD.bazel": `// BUILD file`,
			},
		},
	}

	dir := "."
	for _, testCase := range testCases {
		t.Run(testCase.description, func(t *testing.T) {
			fs := make(map[string][]byte)
			toParse := []string{
				"Android.bp",
			}
			for f, content := range testCase.filesystem {
				if strings.HasSuffix(f, "Android.bp") {
					toParse = append(toParse, f)
				}
				fs[f] = []byte(content)
			}
			config := android.TestConfig(buildDir, nil, testCase.blueprint, fs)
			ctx := android.NewTestContext(config)
			ctx.RegisterModuleType(testCase.moduleTypeUnderTest, testCase.moduleTypeUnderTestFactory)
			ctx.RegisterBp2BuildMutator(testCase.moduleTypeUnderTest, testCase.moduleTypeUnderTestBp2BuildMutator)
			ctx.RegisterForBazelConversion()

			_, errs := ctx.ParseFileList(dir, toParse)
			if errored(t, testCase, errs) {
				return
			}
			_, errs = ctx.ResolveDependencies(config)
			if errored(t, testCase, errs) {
				return
			}

			checkDir := dir
			if testCase.dir != "" {
				checkDir = testCase.dir
			}
			codegenCtx := NewCodegenContext(config, *ctx.Context, Bp2Build)
			bazelTargets, err := generateBazelTargetsForDir(codegenCtx, checkDir)
			android.FailIfErrored(t, err)
			bazelTargets.sort()
			actualCount := len(bazelTargets)
			expectedCount := len(testCase.expectedBazelTargets)
			if actualCount != expectedCount {
				t.Errorf("Expected %d bazel target, got %d\n%s", expectedCount, actualCount, bazelTargets)
			}
			if !strings.Contains(bazelTargets.String(), "Section: Handcrafted targets. ") {
				t.Errorf("Expected string representation of bazelTargets to contain handcrafted section header.")
			}
			for i, target := range bazelTargets {
				actualContent := target.content
				expectedContent := testCase.expectedBazelTargets[i]
				if expectedContent != actualContent {
					t.Errorf(
						"Expected generated Bazel target to be '%s', got '%s'",
						expectedContent,
						actualContent,
					)
				}
			}
		})
	}
}

func TestGlobExcludeSrcs(t *testing.T) {
	testCases := []bp2buildTestCase{
		{
			description:                        "filegroup top level exclude_srcs",
			moduleTypeUnderTest:                "filegroup",
			moduleTypeUnderTestFactory:         android.FileGroupFactory,
			moduleTypeUnderTestBp2BuildMutator: android.FilegroupBp2Build,
			blueprint: `filegroup {
    name: "fg_foo",
    srcs: ["**/*.txt"],
    exclude_srcs: ["c.txt"],
    bazel_module: { bp2build_available: true },
}`,
			expectedBazelTargets: []string{`filegroup(
    name = "fg_foo",
    srcs = [
        "a.txt",
        "b.txt",
        "//dir:e.txt",
        "//dir:f.txt",
    ],
)`,
			},
			filesystem: map[string]string{
				"a.txt":          "",
				"b.txt":          "",
				"c.txt":          "",
				"dir/Android.bp": "",
				"dir/e.txt":      "",
				"dir/f.txt":      "",
			},
		},
		{
			description:                        "filegroup in subdir exclude_srcs",
			moduleTypeUnderTest:                "filegroup",
			moduleTypeUnderTestFactory:         android.FileGroupFactory,
			moduleTypeUnderTestBp2BuildMutator: android.FilegroupBp2Build,
			blueprint:                          "",
			dir:                                "dir",
			filesystem: map[string]string{
				"dir/Android.bp": `filegroup {
    name: "fg_foo",
    srcs: ["**/*.txt"],
    exclude_srcs: ["b.txt"],
    bazel_module: { bp2build_available: true },
}
`,
				"dir/a.txt":             "",
				"dir/b.txt":             "",
				"dir/subdir/Android.bp": "",
				"dir/subdir/e.txt":      "",
				"dir/subdir/f.txt":      "",
			},
			expectedBazelTargets: []string{`filegroup(
    name = "fg_foo",
    srcs = [
        "a.txt",
        "//dir/subdir:e.txt",
        "//dir/subdir:f.txt",
    ],
)`,
			},
		},
	}

	dir := "."
	for _, testCase := range testCases {
		fs := make(map[string][]byte)
		toParse := []string{
			"Android.bp",
		}
		for f, content := range testCase.filesystem {
			if strings.HasSuffix(f, "Android.bp") {
				toParse = append(toParse, f)
			}
			fs[f] = []byte(content)
		}
		config := android.TestConfig(buildDir, nil, testCase.blueprint, fs)
		ctx := android.NewTestContext(config)
		ctx.RegisterModuleType(testCase.moduleTypeUnderTest, testCase.moduleTypeUnderTestFactory)
		ctx.RegisterBp2BuildMutator(testCase.moduleTypeUnderTest, testCase.moduleTypeUnderTestBp2BuildMutator)
		ctx.RegisterForBazelConversion()

		_, errs := ctx.ParseFileList(dir, toParse)
		if errored(t, testCase, errs) {
			continue
		}
		_, errs = ctx.ResolveDependencies(config)
		if errored(t, testCase, errs) {
			continue
		}

		checkDir := dir
		if testCase.dir != "" {
			checkDir = testCase.dir
		}
		codegenCtx := NewCodegenContext(config, *ctx.Context, Bp2Build)
		bazelTargets, err := generateBazelTargetsForDir(codegenCtx, checkDir)
		android.FailIfErrored(t, err)
		if actualCount, expectedCount := len(bazelTargets), len(testCase.expectedBazelTargets); actualCount != expectedCount {
			t.Errorf("%s: Expected %d bazel target, got %d\n%s", testCase.description, expectedCount, actualCount, bazelTargets)
		} else {
			for i, target := range bazelTargets {
				if w, g := testCase.expectedBazelTargets[i], target.content; w != g {
					t.Errorf(
						"%s: Expected generated Bazel target to be '%s', got '%s'",
						testCase.description,
						w,
						g,
					)
				}
			}
		}
	}
}

func TestCommonBp2BuildModuleAttrs(t *testing.T) {
	testCases := []bp2buildTestCase{
		{
			description:                        "Required into data test",
			moduleTypeUnderTest:                "filegroup",
			moduleTypeUnderTestFactory:         android.FileGroupFactory,
			moduleTypeUnderTestBp2BuildMutator: android.FilegroupBp2Build,
			blueprint: `filegroup {
    name: "reqd",
}

filegroup {
    name: "fg_foo",
    required: ["reqd"],
    bazel_module: { bp2build_available: true },
}`,
			expectedBazelTargets: []string{`filegroup(
    name = "fg_foo",
    data = [":reqd"],
)`,
				`filegroup(
    name = "reqd",
)`,
			},
		},
		{
			description:                        "Required via arch into data test",
			moduleTypeUnderTest:                "python_library",
			moduleTypeUnderTestFactory:         python.PythonLibraryFactory,
			moduleTypeUnderTestBp2BuildMutator: python.PythonLibraryBp2Build,
			blueprint: `python_library {
    name: "reqdx86",
    bazel_module: { bp2build_available: false, },
}

python_library {
    name: "reqdarm",
    bazel_module: { bp2build_available: false, },
}

python_library {
    name: "fg_foo",
    arch: {
			 arm: {
				 required: ["reqdarm"],
			 },
			 x86: {
				 required: ["reqdx86"],
			 },
    },
    bazel_module: { bp2build_available: true },
}`,
			expectedBazelTargets: []string{`py_library(
    name = "fg_foo",
    data = select({
        "//build/bazel/platforms/arch:arm": [":reqdarm"],
        "//build/bazel/platforms/arch:x86": [":reqdx86"],
        "//conditions:default": [],
    }),
    srcs_version = "PY3",
)`,
			},
		},
		{
			description:                        "Required appended to data test",
			moduleTypeUnderTest:                "python_library",
			moduleTypeUnderTestFactory:         python.PythonLibraryFactory,
			moduleTypeUnderTestBp2BuildMutator: python.PythonLibraryBp2Build,
			blueprint: `python_library {
    name: "reqd",
    srcs: ["src.py"],
}

python_library {
    name: "fg_foo",
    data: ["data.bin"],
    required: ["reqd"],
    bazel_module: { bp2build_available: true },
}`,
			expectedBazelTargets: []string{
				`py_library(
    name = "fg_foo",
    data = [
        "data.bin",
        ":reqd",
    ],
    srcs_version = "PY3",
)`,
				`py_library(
    name = "reqd",
    srcs = ["src.py"],
    srcs_version = "PY3",
)`,
			},
			filesystem: map[string]string{
				"data.bin": "",
				"src.py":   "",
			},
		},
		{
			description:                        "All props-to-attrs at once together test",
			moduleTypeUnderTest:                "filegroup",
			moduleTypeUnderTestFactory:         android.FileGroupFactory,
			moduleTypeUnderTestBp2BuildMutator: android.FilegroupBp2Build,
			blueprint: `filegroup {
    name: "reqd"
}
filegroup {
    name: "fg_foo",
    required: ["reqd"],
    bazel_module: { bp2build_available: true },
}`,
			expectedBazelTargets: []string{
				`filegroup(
    name = "fg_foo",
    data = [":reqd"],
)`,
				`filegroup(
    name = "reqd",
)`,
			},
			filesystem: map[string]string{},
		},
	}

	for _, test := range testCases {
		runBp2BuildTestCaseSimple(t, test)
	}
}
