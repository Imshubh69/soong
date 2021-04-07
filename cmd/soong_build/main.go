// Copyright 2015 Google Inc. All rights reserved.
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

package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"android/soong/shared"
	"github.com/google/blueprint/bootstrap"

	"android/soong/android"
	"android/soong/bp2build"
)

var (
	topDir            string
	outDir            string
	docFile           string
	bazelQueryViewDir string
	delveListen       string
	delvePath         string
)

func init() {
	flag.StringVar(&topDir, "top", "", "Top directory of the Android source tree")
	flag.StringVar(&outDir, "out", "", "Soong output directory (usually $TOP/out/soong)")
	flag.StringVar(&delveListen, "delve_listen", "", "Delve port to listen on for debugging")
	flag.StringVar(&delvePath, "delve_path", "", "Path to Delve. Only used if --delve_listen is set")
	flag.StringVar(&docFile, "soong_docs", "", "build documentation file to output")
	flag.StringVar(&bazelQueryViewDir, "bazel_queryview_dir", "", "path to the bazel queryview directory relative to --top")
}

func newNameResolver(config android.Config) *android.NameResolver {
	namespacePathsToExport := make(map[string]bool)

	for _, namespaceName := range config.ExportedNamespaces() {
		namespacePathsToExport[namespaceName] = true
	}

	namespacePathsToExport["."] = true // always export the root namespace

	exportFilter := func(namespace *android.Namespace) bool {
		return namespacePathsToExport[namespace.Path]
	}

	return android.NewNameResolver(exportFilter)
}

func newContext(configuration android.Config, prepareBuildActions bool) *android.Context {
	ctx := android.NewContext(configuration)
	ctx.Register()
	if !prepareBuildActions {
		configuration.SetStopBefore(bootstrap.StopBeforePrepareBuildActions)
	}
	ctx.SetNameInterface(newNameResolver(configuration))
	ctx.SetAllowMissingDependencies(configuration.AllowMissingDependencies())
	return ctx
}

func newConfig(srcDir string) android.Config {
	configuration, err := android.NewConfig(srcDir, bootstrap.CmdlineBuildDir(), bootstrap.CmdlineModuleListFile())
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s", err)
		os.Exit(1)
	}
	return configuration
}

// Bazel-enabled mode. Soong runs in two passes.
// First pass: Analyze the build tree, but only store all bazel commands
// needed to correctly evaluate the tree in the second pass.
// TODO(cparsons): Don't output any ninja file, as the second pass will overwrite
// the incorrect results from the first pass, and file I/O is expensive.
func runMixedModeBuild(configuration android.Config, firstCtx *android.Context, extraNinjaDeps []string) {
	configuration.SetStopBefore(bootstrap.StopBeforeWriteNinja)
	bootstrap.Main(firstCtx.Context, configuration, false, extraNinjaDeps...)
	// Invoke bazel commands and save results for second pass.
	if err := configuration.BazelContext.InvokeBazel(); err != nil {
		fmt.Fprintf(os.Stderr, "%s", err)
		os.Exit(1)
	}
	// Second pass: Full analysis, using the bazel command results. Output ninja file.
	secondPassConfig, err := android.ConfigForAdditionalRun(configuration)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s", err)
		os.Exit(1)
	}
	secondCtx := newContext(secondPassConfig, true)
	bootstrap.Main(secondCtx.Context, secondPassConfig, false, extraNinjaDeps...)
}

// Run the code-generation phase to convert BazelTargetModules to BUILD files.
func runQueryView(configuration android.Config, ctx *android.Context) {
	codegenContext := bp2build.NewCodegenContext(configuration, *ctx, bp2build.QueryView)
	absoluteQueryViewDir := shared.JoinPath(topDir, bazelQueryViewDir)
	if err := createBazelQueryView(codegenContext, absoluteQueryViewDir); err != nil {
		fmt.Fprintf(os.Stderr, "%s", err)
		os.Exit(1)
	}
}

func runSoongDocs(configuration android.Config, extraNinjaDeps []string) {
	ctx := newContext(configuration, false)
	bootstrap.Main(ctx.Context, configuration, false, extraNinjaDeps...)
	if err := writeDocs(ctx, configuration, docFile); err != nil {
		fmt.Fprintf(os.Stderr, "%s", err)
		os.Exit(1)
	}
}

func writeMetrics(configuration android.Config) {
	metricsFile := filepath.Join(bootstrap.CmdlineBuildDir(), "soong_build_metrics.pb")
	err := android.WriteMetrics(configuration, metricsFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error writing soong_build metrics %s: %s", metricsFile, err)
		os.Exit(1)
	}
}

func writeJsonModuleGraph(configuration android.Config, ctx *android.Context, path string, extraNinjaDeps []string) {
	f, err := os.Create(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s", err)
		os.Exit(1)
	}

	defer f.Close()
	ctx.Context.PrintJSONGraph(f)
	writeFakeNinjaFile(extraNinjaDeps, configuration.BuildDir())
}

func doChosenActivity(configuration android.Config, extraNinjaDeps []string) {
	bazelConversionRequested := configuration.IsEnvTrue("GENERATE_BAZEL_FILES")
	mixedModeBuild := configuration.BazelContext.BazelEnabled()
	generateQueryView := bazelQueryViewDir != ""
	jsonModuleFile := configuration.Getenv("SOONG_DUMP_JSON_MODULE_GRAPH")

	prepareBuildActions := !generateQueryView && jsonModuleFile == ""

	if bazelConversionRequested {
		// Run the alternate pipeline of bp2build mutators and singleton to convert
		// Blueprint to BUILD files before everything else.
		runBp2Build(configuration, extraNinjaDeps)
		return
	}

	ctx := newContext(configuration, prepareBuildActions)
	if mixedModeBuild {
		runMixedModeBuild(configuration, ctx, extraNinjaDeps)
	} else {
		bootstrap.Main(ctx.Context, configuration, false, extraNinjaDeps...)
	}

	// Convert the Soong module graph into Bazel BUILD files.
	if generateQueryView {
		runQueryView(configuration, ctx)
		return
	}

	if jsonModuleFile != "" {
		writeJsonModuleGraph(configuration, ctx, jsonModuleFile, extraNinjaDeps)
		return
	}

	writeMetrics(configuration)
}

func main() {
	flag.Parse()

	shared.ReexecWithDelveMaybe(delveListen, delvePath)
	android.InitSandbox(topDir)
	android.InitEnvironment(shared.JoinPath(topDir, outDir, "soong.environment.available"))

	usedVariablesFile := shared.JoinPath(outDir, "soong.environment.used")
	// The top-level Blueprints file is passed as the first argument.
	srcDir := filepath.Dir(flag.Arg(0))
	configuration := newConfig(srcDir)
	extraNinjaDeps := []string{
		configuration.ProductVariablesFileName,
		shared.JoinPath(outDir, "soong.environment.used"),
	}

	if configuration.Getenv("ALLOW_MISSING_DEPENDENCIES") == "true" {
		configuration.SetAllowMissingDependencies()
	}

	// These two are here so that we restart a non-debugged soong_build when the
	// user sets SOONG_DELVE the first time.
	configuration.Getenv("SOONG_DELVE")
	configuration.Getenv("SOONG_DELVE_PATH")
	if shared.IsDebugging() {
		// Add a non-existent file to the dependencies so that soong_build will rerun when the debugger is
		// enabled even if it completed successfully.
		extraNinjaDeps = append(extraNinjaDeps, filepath.Join(configuration.BuildDir(), "always_rerun_for_delve"))
	}

	if docFile != "" {
		// We don't write an used variables file when generating documentation
		// because that is done from within the actual builds as a Ninja action and
		// thus it would overwrite the actual used variables file so this is
		// special-cased.
		runSoongDocs(configuration, extraNinjaDeps)
		return
	}

	doChosenActivity(configuration, extraNinjaDeps)
	writeUsedVariablesFile(shared.JoinPath(topDir, usedVariablesFile), configuration)
}

func writeUsedVariablesFile(path string, configuration android.Config) {
	data, err := shared.EnvFileContents(configuration.EnvDeps())
	if err != nil {
		fmt.Fprintf(os.Stderr, "error writing used variables file %s: %s\n", path, err)
		os.Exit(1)
	}

	err = ioutil.WriteFile(path, data, 0666)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error writing used variables file %s: %s\n", path, err)
		os.Exit(1)
	}

	// Touch the output Ninja file so that it's not older than the file we just
	// wrote. We can't write the environment file earlier because one an access
	// new environment variables while writing it.
	outputNinjaFile := shared.JoinPath(topDir, bootstrap.CmdlineOutFile())
	currentTime := time.Now().Local()
	err = os.Chtimes(outputNinjaFile, currentTime, currentTime)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error touching output file %s: %s\n", outputNinjaFile, err)
		os.Exit(1)
	}
}

// Workarounds to support running bp2build in a clean AOSP checkout with no
// prior builds, and exiting early as soon as the BUILD files get generated,
// therefore not creating build.ninja files that soong_ui and callers of
// soong_build expects.
//
// These files are: build.ninja and build.ninja.d. Since Kati hasn't been
// ran as well, and `nothing` is defined in a .mk file, there isn't a ninja
// target called `nothing`, so we manually create it here.
func writeFakeNinjaFile(extraNinjaDeps []string, buildDir string) {
	extraNinjaDepsString := strings.Join(extraNinjaDeps, " \\\n ")

	ninjaFileName := "build.ninja"
	ninjaFile := shared.JoinPath(topDir, buildDir, ninjaFileName)
	ninjaFileD := shared.JoinPath(topDir, buildDir, ninjaFileName)
	// A workaround to create the 'nothing' ninja target so `m nothing` works,
	// since bp2build runs without Kati, and the 'nothing' target is declared in
	// a Makefile.
	ioutil.WriteFile(ninjaFile, []byte("build nothing: phony\n  phony_output = true\n"), 0666)
	ioutil.WriteFile(ninjaFileD,
		[]byte(fmt.Sprintf("%s: \\\n %s\n", ninjaFileName, extraNinjaDepsString)),
		0666)
}

// Run Soong in the bp2build mode. This creates a standalone context that registers
// an alternate pipeline of mutators and singletons specifically for generating
// Bazel BUILD files instead of Ninja files.
func runBp2Build(configuration android.Config, extraNinjaDeps []string) {
	// Register an alternate set of singletons and mutators for bazel
	// conversion for Bazel conversion.
	bp2buildCtx := android.NewContext(configuration)
	bp2buildCtx.RegisterForBazelConversion()

	// No need to generate Ninja build rules/statements from Modules and Singletons.
	configuration.SetStopBefore(bootstrap.StopBeforePrepareBuildActions)
	bp2buildCtx.SetNameInterface(newNameResolver(configuration))

	// The bp2build process is a purely functional process that only depends on
	// Android.bp files. It must not depend on the values of per-build product
	// configurations or variables, since those will generate different BUILD
	// files based on how the user has configured their tree.
	bp2buildCtx.SetModuleListFile(bootstrap.CmdlineModuleListFile())
	modulePaths, err := bp2buildCtx.ListModulePaths(configuration.SrcDir())
	if err != nil {
		panic(err)
	}

	extraNinjaDeps = append(extraNinjaDeps, modulePaths...)

	// Run the loading and analysis pipeline to prepare the graph of regular
	// Modules parsed from Android.bp files, and the BazelTargetModules mapped
	// from the regular Modules.
	bootstrap.Main(bp2buildCtx.Context, configuration, false, extraNinjaDeps...)

	// Run the code-generation phase to convert BazelTargetModules to BUILD files
	// and print conversion metrics to the user.
	codegenContext := bp2build.NewCodegenContext(configuration, *bp2buildCtx, bp2build.Bp2Build)
	metrics := bp2build.Codegen(codegenContext)

	// Only report metrics when in bp2build mode. The metrics aren't relevant
	// for queryview, since that's a total repo-wide conversion and there's a
	// 1:1 mapping for each module.
	metrics.Print()

	extraNinjaDeps = append(extraNinjaDeps, codegenContext.AdditionalNinjaDeps()...)
	writeFakeNinjaFile(extraNinjaDeps, codegenContext.Config().BuildDir())
}
