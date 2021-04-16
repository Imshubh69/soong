// Copyright 2019 Google Inc. All rights reserved.
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

package java

import (
	"android/soong/android"
)

func init() {
	RegisterHiddenApiSingletonComponents(android.InitRegistrationContext)
}

func RegisterHiddenApiSingletonComponents(ctx android.RegistrationContext) {
	ctx.RegisterSingletonType("hiddenapi", hiddenAPISingletonFactory)
}

var PrepareForTestWithHiddenApiBuildComponents = android.FixtureRegisterWithContext(RegisterHiddenApiSingletonComponents)

type hiddenAPISingletonPathsStruct struct {
	// The path to the CSV file that contains the flags that will be encoded into the dex boot jars.
	//
	// It is created by the generate_hiddenapi_lists.py tool that is passed the stubFlags along with
	// a number of additional files that are used to augment the information in the stubFlags with
	// manually curated data.
	flags android.OutputPath

	// The path to the CSV index file that contains mappings from Java signature to source location
	// information for all Java elements annotated with the UnsupportedAppUsage annotation in the
	// source of all the boot jars.
	//
	// It is created by the merge_csv tool which merges all the hiddenAPI.indexCSVPath files that have
	// been created by the rest of the build. That includes the index files generated for
	// <x>-hiddenapi modules.
	index android.OutputPath

	// The path to the CSV metadata file that contains mappings from Java signature to the value of
	// properties specified on UnsupportedAppUsage annotations in the source of all the boot jars.
	//
	// It is created by the merge_csv tool which merges all the hiddenAPI.metadataCSVPath files that
	// have been created by the rest of the build. That includes the metadata files generated for
	// <x>-hiddenapi modules.
	metadata android.OutputPath

	// The path to the CSV metadata file that contains mappings from Java signature to flags obtained
	// from the public, system and test API stubs.
	//
	// This is created by the hiddenapi tool which is given dex files for the public, system and test
	// API stubs (including product specific stubs) along with dex boot jars, so does not include
	// <x>-hiddenapi modules. For each API surface (i.e. public, system, test) it records which
	// members in the dex boot jars match a member in the dex stub jars for that API surface and then
	// outputs a file containing the signatures of all members in the dex boot jars along with the
	// flags that indicate which API surface it belongs, if any.
	//
	// e.g. a dex member that matches a member in the public dex stubs would have flags
	// "public-api,system-api,test-api" set (as system and test are both supersets of public). A dex
	// member that didn't match a member in any of the dex stubs is still output it just has an empty
	// set of flags.
	//
	// The notion of matching is quite complex, it is not restricted to just exact matching but also
	// follows the Java inheritance rules. e.g. if a method is public then all overriding/implementing
	// methods are also public. If an interface method is public and a class inherits an
	// implementation of that method from a super class then that super class method is also public.
	// That ensures that any method that can be called directly by an App through a public method is
	// visible to that App.
	//
	// Propagating the visibility of members across the inheritance hierarchy at build time will cause
	// problems when modularizing and unbundling as it that propagation can cross module boundaries.
	// e.g. Say that a private framework class implements a public interface and inherits an
	// implementation of one of its methods from a core platform ART class. In that case the ART
	// implementation method needs to be marked as public which requires the build to have access to
	// the framework implementation classes at build time. The work to rectify this is being tracked
	// at http://b/178693149.
	//
	// This file (or at least those items marked as being in the public-api) is used by hiddenapi when
	// creating the metadata and flags for the individual modules in order to perform consistency
	// checks and filter out bridge methods that are part of the public API. The latter relies on the
	// propagation of visibility across the inheritance hierarchy.
	stubFlags android.OutputPath
}

var hiddenAPISingletonPathsKey = android.NewOnceKey("hiddenAPISingletonPathsKey")

// hiddenAPISingletonPaths creates all the paths for singleton files the first time it is called, which may be
// from a ModuleContext that needs to reference a file that will be created by a singleton rule that hasn't
// yet been created.
func hiddenAPISingletonPaths(ctx android.PathContext) hiddenAPISingletonPathsStruct {
	return ctx.Config().Once(hiddenAPISingletonPathsKey, func() interface{} {
		// Make the paths relative to the out/soong/hiddenapi directory instead of to the out/soong/
		// directory. This ensures that if they are used as java_resources they do not end up in a
		// hiddenapi directory in the resulting APK.
		hiddenapiDir := android.PathForOutput(ctx, "hiddenapi")
		return hiddenAPISingletonPathsStruct{
			flags:     hiddenapiDir.Join(ctx, "hiddenapi-flags.csv"),
			index:     hiddenapiDir.Join(ctx, "hiddenapi-index.csv"),
			metadata:  hiddenapiDir.Join(ctx, "hiddenapi-unsupported.csv"),
			stubFlags: hiddenapiDir.Join(ctx, "hiddenapi-stub-flags.txt"),
		}
	}).(hiddenAPISingletonPathsStruct)
}

func hiddenAPISingletonFactory() android.Singleton {
	return &hiddenAPISingleton{}
}

type hiddenAPISingleton struct {
	flags android.Path
}

// hiddenAPI singleton rules
func (h *hiddenAPISingleton) GenerateBuildActions(ctx android.SingletonContext) {
	// Don't run any hiddenapi rules if UNSAFE_DISABLE_HIDDENAPI_FLAGS=true
	if ctx.Config().IsEnvTrue("UNSAFE_DISABLE_HIDDENAPI_FLAGS") {
		return
	}

	stubFlagsRule(ctx)

	// If there is a prebuilt hiddenapi dir, generate rules to use the
	// files within. Generally, we build the hiddenapi files from source
	// during the build, ensuring consistency. It's possible, in a split
	// build (framework and vendor) scenario, for the vendor build to use
	// prebuilt hiddenapi files from the framework build. In this scenario,
	// the framework and vendor builds must use the same source to ensure
	// consistency.

	if ctx.Config().PrebuiltHiddenApiDir(ctx) != "" {
		h.flags = prebuiltFlagsRule(ctx)
		prebuiltIndexRule(ctx)
		return
	}

	// These rules depend on files located in frameworks/base, skip them if running in a tree that doesn't have them.
	if ctx.Config().FrameworksBaseDirExists(ctx) {
		h.flags = flagsRule(ctx)
	} else {
		h.flags = emptyFlagsRule(ctx)
	}
}

// Export paths to Make.  INTERNAL_PLATFORM_HIDDENAPI_FLAGS is used by Make rules in art/ and cts/.
func (h *hiddenAPISingleton) MakeVars(ctx android.MakeVarsContext) {
	if ctx.Config().IsEnvTrue("UNSAFE_DISABLE_HIDDENAPI_FLAGS") {
		return
	}

	ctx.Strict("INTERNAL_PLATFORM_HIDDENAPI_FLAGS", h.flags.String())
}

// stubFlagsRule creates the rule to build hiddenapi-stub-flags.txt out of dex jars from stub modules and boot image
// modules.
func stubFlagsRule(ctx android.SingletonContext) {
	var publicStubModules []string
	var systemStubModules []string
	var testStubModules []string
	var corePlatformStubModules []string

	if ctx.Config().AlwaysUsePrebuiltSdks() {
		// Build configuration mandates using prebuilt stub modules
		publicStubModules = append(publicStubModules, "sdk_public_current_android")
		systemStubModules = append(systemStubModules, "sdk_system_current_android")
		testStubModules = append(testStubModules, "sdk_test_current_android")
	} else {
		// Use stub modules built from source
		publicStubModules = append(publicStubModules, "android_stubs_current")
		systemStubModules = append(systemStubModules, "android_system_stubs_current")
		testStubModules = append(testStubModules, "android_test_stubs_current")
	}
	// We do not have prebuilts of the core platform api yet
	corePlatformStubModules = append(corePlatformStubModules, "legacy.core.platform.api.stubs")

	// Allow products to define their own stubs for custom product jars that apps can use.
	publicStubModules = append(publicStubModules, ctx.Config().ProductHiddenAPIStubs()...)
	systemStubModules = append(systemStubModules, ctx.Config().ProductHiddenAPIStubsSystem()...)
	testStubModules = append(testStubModules, ctx.Config().ProductHiddenAPIStubsTest()...)
	if ctx.Config().IsEnvTrue("EMMA_INSTRUMENT") {
		publicStubModules = append(publicStubModules, "jacoco-stubs")
	}

	publicStubPaths := make(android.Paths, len(publicStubModules))
	systemStubPaths := make(android.Paths, len(systemStubModules))
	testStubPaths := make(android.Paths, len(testStubModules))
	corePlatformStubPaths := make(android.Paths, len(corePlatformStubModules))

	moduleListToPathList := map[*[]string]android.Paths{
		&publicStubModules:       publicStubPaths,
		&systemStubModules:       systemStubPaths,
		&testStubModules:         testStubPaths,
		&corePlatformStubModules: corePlatformStubPaths,
	}

	var bootDexJars android.Paths

	ctx.VisitAllModules(func(module android.Module) {
		// Collect dex jar paths for the modules listed above.
		if j, ok := module.(UsesLibraryDependency); ok {
			name := ctx.ModuleName(module)
			for moduleList, pathList := range moduleListToPathList {
				if i := android.IndexList(name, *moduleList); i != -1 {
					pathList[i] = j.DexJarBuildPath()
				}
			}
		}

		// Collect dex jar paths for modules that had hiddenapi encode called on them.
		if h, ok := module.(hiddenAPIIntf); ok {
			if jar := h.bootDexJar(); jar != nil {
				bootDexJars = append(bootDexJars, jar)
			}
		}
	})

	var missingDeps []string
	// Ensure all modules were converted to paths
	for moduleList, pathList := range moduleListToPathList {
		for i := range pathList {
			if pathList[i] == nil {
				moduleName := (*moduleList)[i]
				pathList[i] = android.PathForOutput(ctx, "missing/module", moduleName)
				if ctx.Config().AllowMissingDependencies() {
					missingDeps = append(missingDeps, moduleName)
				} else {
					ctx.Errorf("failed to find dex jar path for module %q",
						moduleName)
				}
			}
		}
	}

	// Singleton rule which applies hiddenapi on all boot class path dex files.
	rule := android.NewRuleBuilder(pctx, ctx)

	outputPath := hiddenAPISingletonPaths(ctx).stubFlags
	tempPath := android.PathForOutput(ctx, outputPath.Rel()+".tmp")

	rule.MissingDeps(missingDeps)

	rule.Command().
		Tool(ctx.Config().HostToolPath(ctx, "hiddenapi")).
		Text("list").
		FlagForEachInput("--boot-dex=", bootDexJars).
		FlagWithInputList("--public-stub-classpath=", publicStubPaths, ":").
		FlagWithInputList("--system-stub-classpath=", systemStubPaths, ":").
		FlagWithInputList("--test-stub-classpath=", testStubPaths, ":").
		FlagWithInputList("--core-platform-stub-classpath=", corePlatformStubPaths, ":").
		FlagWithOutput("--out-api-flags=", tempPath)

	commitChangeForRestat(rule, tempPath, outputPath)

	rule.Build("hiddenAPIStubFlagsFile", "hiddenapi stub flags")
}

// Checks to see whether the supplied module variant is in the list of boot jars.
//
// This is similar to logic in getBootImageJar() so any changes needed here are likely to be needed
// there too.
//
// TODO(b/179354495): Avoid having to perform this type of check or if necessary dedup it.
func isModuleInConfiguredList(ctx android.BaseModuleContext, module android.Module, configuredBootJars android.ConfiguredJarList) bool {
	name := ctx.OtherModuleName(module)

	// Strip a prebuilt_ prefix so that this can match a prebuilt module that has not been renamed.
	name = android.RemoveOptionalPrebuiltPrefix(name)

	// Ignore any module that is not listed in the boot image configuration.
	index := configuredBootJars.IndexOfJar(name)
	if index == -1 {
		return false
	}

	// It is an error if the module is not an ApexModule.
	if _, ok := module.(android.ApexModule); !ok {
		ctx.ModuleErrorf("is configured in boot jars but does not support being added to an apex")
		return false
	}

	apexInfo := ctx.OtherModuleProvider(module, android.ApexInfoProvider).(android.ApexInfo)

	// Now match the apex part of the boot image configuration.
	requiredApex := configuredBootJars.Apex(index)
	if requiredApex == "platform" {
		if len(apexInfo.InApexes) != 0 {
			// A platform variant is required but this is for an apex so ignore it.
			return false
		}
	} else if !apexInfo.InApexByBaseName(requiredApex) {
		// An apex variant for a specific apex is required but this is the wrong apex.
		return false
	}

	return true
}

func prebuiltFlagsRule(ctx android.SingletonContext) android.Path {
	outputPath := hiddenAPISingletonPaths(ctx).flags
	inputPath := android.PathForSource(ctx, ctx.Config().PrebuiltHiddenApiDir(ctx), "hiddenapi-flags.csv")

	ctx.Build(pctx, android.BuildParams{
		Rule:   android.Cp,
		Output: outputPath,
		Input:  inputPath,
	})

	return outputPath
}

func prebuiltIndexRule(ctx android.SingletonContext) {
	outputPath := hiddenAPISingletonPaths(ctx).index
	inputPath := android.PathForSource(ctx, ctx.Config().PrebuiltHiddenApiDir(ctx), "hiddenapi-index.csv")

	ctx.Build(pctx, android.BuildParams{
		Rule:   android.Cp,
		Output: outputPath,
		Input:  inputPath,
	})
}

// flagsRule is a placeholder that simply returns the location of the file, the generation of the
// ninja rules is done in generateHiddenAPIBuildActions.
func flagsRule(ctx android.SingletonContext) android.Path {
	outputPath := hiddenAPISingletonPaths(ctx).flags
	return outputPath
}

// emptyFlagsRule creates a rule to build an empty hiddenapi-flags.csv, which is needed by master-art-host builds that
// have a partial manifest without frameworks/base but still need to build a boot image.
func emptyFlagsRule(ctx android.SingletonContext) android.Path {
	rule := android.NewRuleBuilder(pctx, ctx)

	outputPath := hiddenAPISingletonPaths(ctx).flags

	rule.Command().Text("rm").Flag("-f").Output(outputPath)
	rule.Command().Text("touch").Output(outputPath)

	rule.Build("emptyHiddenAPIFlagsFile", "empty hiddenapi flags")

	return outputPath
}

// commitChangeForRestat adds a command to a rule that updates outputPath from tempPath if they are different.  It
// also marks the rule as restat and marks the tempPath as a temporary file that should not be considered an output of
// the rule.
func commitChangeForRestat(rule *android.RuleBuilder, tempPath, outputPath android.WritablePath) {
	rule.Restat()
	rule.Temporary(tempPath)
	rule.Command().
		Text("(").
		Text("if").
		Text("cmp -s").Input(tempPath).Output(outputPath).Text(";").
		Text("then").
		Text("rm").Input(tempPath).Text(";").
		Text("else").
		Text("mv").Input(tempPath).Output(outputPath).Text(";").
		Text("fi").
		Text(")")
}
