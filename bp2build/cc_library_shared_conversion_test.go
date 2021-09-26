// Copyright 2021 Google Inc. All rights reserved.
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
	"testing"

	"android/soong/android"
	"android/soong/cc"
)

const (
	// See cc/testing.go for more context
	// TODO(alexmarquez): Split out the preamble into common code?
	soongCcLibrarySharedPreamble = soongCcLibraryStaticPreamble
)

func registerCcLibrarySharedModuleTypes(ctx android.RegistrationContext) {
	cc.RegisterCCBuildComponents(ctx)
	ctx.RegisterModuleType("toolchain_library", cc.ToolchainLibraryFactory)
	ctx.RegisterModuleType("cc_library_headers", cc.LibraryHeaderFactory)
	ctx.RegisterModuleType("cc_library_static", cc.LibraryStaticFactory)
}

func runCcLibrarySharedTestCase(t *testing.T, tc bp2buildTestCase) {
	t.Helper()
	runBp2BuildTestCase(t, registerCcLibrarySharedModuleTypes, tc)
}

func TestCcLibrarySharedSimple(t *testing.T) {
	runCcLibrarySharedTestCase(t, bp2buildTestCase{
		description:                        "cc_library_shared simple overall test",
		moduleTypeUnderTest:                "cc_library_shared",
		moduleTypeUnderTestFactory:         cc.LibrarySharedFactory,
		moduleTypeUnderTestBp2BuildMutator: cc.CcLibrarySharedBp2Build,
		filesystem: map[string]string{
			// NOTE: include_dir headers *should not* appear in Bazel hdrs later (?)
			"include_dir_1/include_dir_1_a.h": "",
			"include_dir_1/include_dir_1_b.h": "",
			"include_dir_2/include_dir_2_a.h": "",
			"include_dir_2/include_dir_2_b.h": "",
			// NOTE: local_include_dir headers *should not* appear in Bazel hdrs later (?)
			"local_include_dir_1/local_include_dir_1_a.h": "",
			"local_include_dir_1/local_include_dir_1_b.h": "",
			"local_include_dir_2/local_include_dir_2_a.h": "",
			"local_include_dir_2/local_include_dir_2_b.h": "",
			// NOTE: export_include_dir headers *should* appear in Bazel hdrs later
			"export_include_dir_1/export_include_dir_1_a.h": "",
			"export_include_dir_1/export_include_dir_1_b.h": "",
			"export_include_dir_2/export_include_dir_2_a.h": "",
			"export_include_dir_2/export_include_dir_2_b.h": "",
			// NOTE: Soong implicitly includes headers in the current directory
			"implicit_include_1.h": "",
			"implicit_include_2.h": "",
		},
		blueprint: soongCcLibrarySharedPreamble + `
cc_library_headers {
    name: "header_lib_1",
    export_include_dirs: ["header_lib_1"],
    bazel_module: { bp2build_available: false },
}

cc_library_headers {
    name: "header_lib_2",
    export_include_dirs: ["header_lib_2"],
    bazel_module: { bp2build_available: false },
}

cc_library_shared {
    name: "shared_lib_1",
    srcs: ["shared_lib_1.cc"],
    bazel_module: { bp2build_available: false },
}

cc_library_shared {
    name: "shared_lib_2",
    srcs: ["shared_lib_2.cc"],
    bazel_module: { bp2build_available: false },
}

cc_library_static {
    name: "whole_static_lib_1",
    srcs: ["whole_static_lib_1.cc"],
    bazel_module: { bp2build_available: false },
}

cc_library_static {
    name: "whole_static_lib_2",
    srcs: ["whole_static_lib_2.cc"],
    bazel_module: { bp2build_available: false },
}

cc_library_shared {
    name: "foo_shared",
    srcs: [
        "foo_shared1.cc",
        "foo_shared2.cc",
    ],
    cflags: [
        "-Dflag1",
        "-Dflag2"
    ],
    shared_libs: [
        "shared_lib_1",
        "shared_lib_2"
    ],
    whole_static_libs: [
        "whole_static_lib_1",
        "whole_static_lib_2"
    ],
    include_dirs: [
        "include_dir_1",
        "include_dir_2",
    ],
    local_include_dirs: [
        "local_include_dir_1",
        "local_include_dir_2",
    ],
    export_include_dirs: [
        "export_include_dir_1",
        "export_include_dir_2"
    ],
    header_libs: [
        "header_lib_1",
        "header_lib_2"
    ],

    // TODO: Also support export_header_lib_headers
}`,
		expectedBazelTargets: []string{`cc_library_shared(
    name = "foo_shared",
    absolute_includes = [
        "include_dir_1",
        "include_dir_2",
    ],
    copts = [
        "-Dflag1",
        "-Dflag2",
    ],
    export_includes = [
        "export_include_dir_1",
        "export_include_dir_2",
    ],
    implementation_deps = [
        ":header_lib_1",
        ":header_lib_2",
    ],
    implementation_dynamic_deps = [
        ":shared_lib_1",
        ":shared_lib_2",
    ],
    local_includes = [
        "local_include_dir_1",
        "local_include_dir_2",
        ".",
    ],
    srcs = [
        "foo_shared1.cc",
        "foo_shared2.cc",
    ],
    whole_archive_deps = [
        ":whole_static_lib_1",
        ":whole_static_lib_2",
    ],
)`},
	})
}

func TestCcLibrarySharedArchSpecificSharedLib(t *testing.T) {
	runCcLibrarySharedTestCase(t, bp2buildTestCase{
		description:                        "cc_library_shared arch-specific shared_libs with whole_static_libs",
		moduleTypeUnderTest:                "cc_library_shared",
		moduleTypeUnderTestFactory:         cc.LibrarySharedFactory,
		moduleTypeUnderTestBp2BuildMutator: cc.CcLibrarySharedBp2Build,
		filesystem:                         map[string]string{},
		blueprint: soongCcLibrarySharedPreamble + `
cc_library_static {
    name: "static_dep",
    bazel_module: { bp2build_available: false },
}
cc_library_shared {
    name: "shared_dep",
    bazel_module: { bp2build_available: false },
}
cc_library_shared {
    name: "foo_shared",
    arch: { arm64: { shared_libs: ["shared_dep"], whole_static_libs: ["static_dep"] } },
    include_build_directory: false,
}`,
		expectedBazelTargets: []string{`cc_library_shared(
    name = "foo_shared",
    implementation_dynamic_deps = select({
        "//build/bazel/platforms/arch:arm64": [":shared_dep"],
        "//conditions:default": [],
    }),
    whole_archive_deps = select({
        "//build/bazel/platforms/arch:arm64": [":static_dep"],
        "//conditions:default": [],
    }),
)`},
	})
}

func TestCcLibrarySharedOsSpecificSharedLib(t *testing.T) {
	runCcLibraryStaticTestCase(t, bp2buildTestCase{
		description:                        "cc_library_shared os-specific shared_libs",
		moduleTypeUnderTest:                "cc_library_shared",
		moduleTypeUnderTestFactory:         cc.LibrarySharedFactory,
		moduleTypeUnderTestBp2BuildMutator: cc.CcLibrarySharedBp2Build,
		filesystem:                         map[string]string{},
		blueprint: soongCcLibrarySharedPreamble + `
cc_library_shared {
    name: "shared_dep",
    bazel_module: { bp2build_available: false },
}
cc_library_shared {
    name: "foo_shared",
    target: { android: { shared_libs: ["shared_dep"], } },
    include_build_directory: false,
}`,
		expectedBazelTargets: []string{`cc_library_shared(
    name = "foo_shared",
    implementation_dynamic_deps = select({
        "//build/bazel/platforms/os:android": [":shared_dep"],
        "//conditions:default": [],
    }),
)`},
	})
}

func TestCcLibrarySharedBaseArchOsSpecificSharedLib(t *testing.T) {
	runCcLibrarySharedTestCase(t, bp2buildTestCase{
		description:                        "cc_library_shared base, arch, and os-specific shared_libs",
		moduleTypeUnderTest:                "cc_library_shared",
		moduleTypeUnderTestFactory:         cc.LibrarySharedFactory,
		moduleTypeUnderTestBp2BuildMutator: cc.CcLibrarySharedBp2Build,
		filesystem:                         map[string]string{},
		blueprint: soongCcLibrarySharedPreamble + `
cc_library_shared {
    name: "shared_dep",
    bazel_module: { bp2build_available: false },
}
cc_library_shared {
    name: "shared_dep2",
    bazel_module: { bp2build_available: false },
}
cc_library_shared {
    name: "shared_dep3",
    bazel_module: { bp2build_available: false },
}
cc_library_shared {
    name: "foo_shared",
    shared_libs: ["shared_dep"],
    target: { android: { shared_libs: ["shared_dep2"] } },
    arch: { arm64: { shared_libs: ["shared_dep3"] } },
    include_build_directory: false,
}`,
		expectedBazelTargets: []string{`cc_library_shared(
    name = "foo_shared",
    implementation_dynamic_deps = [":shared_dep"] + select({
        "//build/bazel/platforms/arch:arm64": [":shared_dep3"],
        "//conditions:default": [],
    }) + select({
        "//build/bazel/platforms/os:android": [":shared_dep2"],
        "//conditions:default": [],
    }),
)`},
	})
}

func TestCcLibrarySharedSimpleExcludeSrcs(t *testing.T) {
	runCcLibrarySharedTestCase(t, bp2buildTestCase{
		description:                        "cc_library_shared simple exclude_srcs",
		moduleTypeUnderTest:                "cc_library_shared",
		moduleTypeUnderTestFactory:         cc.LibrarySharedFactory,
		moduleTypeUnderTestBp2BuildMutator: cc.CcLibrarySharedBp2Build,
		filesystem: map[string]string{
			"common.c":       "",
			"foo-a.c":        "",
			"foo-excluded.c": "",
		},
		blueprint: soongCcLibrarySharedPreamble + `
cc_library_shared {
    name: "foo_shared",
    srcs: ["common.c", "foo-*.c"],
    exclude_srcs: ["foo-excluded.c"],
    include_build_directory: false,
}`,
		expectedBazelTargets: []string{`cc_library_shared(
    name = "foo_shared",
    srcs_c = [
        "common.c",
        "foo-a.c",
    ],
)`},
	})
}

func TestCcLibrarySharedStrip(t *testing.T) {
	runCcLibrarySharedTestCase(t, bp2buildTestCase{
		description:                        "cc_library_shared stripping",
		moduleTypeUnderTest:                "cc_library_shared",
		moduleTypeUnderTestFactory:         cc.LibrarySharedFactory,
		moduleTypeUnderTestBp2BuildMutator: cc.CcLibrarySharedBp2Build,
		filesystem:                         map[string]string{},
		blueprint: soongCcLibrarySharedPreamble + `
cc_library_shared {
    name: "foo_shared",
    strip: {
        keep_symbols: false,
        keep_symbols_and_debug_frame: true,
        keep_symbols_list: ["sym", "sym2"],
        all: true,
        none: false,
    },
    include_build_directory: false,
}`,
		expectedBazelTargets: []string{`cc_library_shared(
    name = "foo_shared",
    strip = {
        "all": True,
        "keep_symbols": False,
        "keep_symbols_and_debug_frame": True,
        "keep_symbols_list": [
            "sym",
            "sym2",
        ],
        "none": False,
    },
)`},
	})
}

func TestCcLibrarySharedVersionScript(t *testing.T) {
	runCcLibrarySharedTestCase(t, bp2buildTestCase{
		description:                        "cc_library_shared version script",
		moduleTypeUnderTest:                "cc_library_shared",
		moduleTypeUnderTestFactory:         cc.LibrarySharedFactory,
		moduleTypeUnderTestBp2BuildMutator: cc.CcLibrarySharedBp2Build,
		filesystem: map[string]string{
			"version_script": "",
		},
		blueprint: soongCcLibrarySharedPreamble + `
cc_library_shared {
    name: "foo_shared",
    version_script: "version_script",
    include_build_directory: false,
}`,
		expectedBazelTargets: []string{`cc_library_shared(
    name = "foo_shared",
    version_script = "version_script",
)`},
	})
}
