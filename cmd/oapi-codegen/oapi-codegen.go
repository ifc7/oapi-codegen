// Copyright 2019 DeepMap, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/oapi-codegen/oapi-codegen/v2/pkg/codegen"
	"github.com/oapi-codegen/oapi-codegen/v2/pkg/util"
)

func errExit(format string, args ...any) {
	if !strings.HasSuffix(format, "\n") {
		format = format + "\n"
	}
	_, _ = fmt.Fprintf(os.Stderr, format, args...)
	os.Exit(1)
}

var (
	flagOutputFile   string
	flagConfigFile   string
	flagOutputConfig bool
	flagPrintVersion bool
	flagPackageName  string
	flagPrintUsage   bool
)

type configuration struct {
	codegen.Configuration `yaml:",inline"`

	// OutputFile is the filename to output.
	OutputFile string `yaml:"output,omitempty"`
}

// noVCSVersionOverride allows overriding the version of the application for cases where no Version Control System (VCS) is available when building, for instance when using a Nix derivation.
// See documentation for how to use it in examples/no-vcs-version-override/README.md
var noVCSVersionOverride string

func main() {
	flag.StringVar(&flagOutputFile, "o", "", "Where to output generated code, stdout is default.")
	flag.BoolVar(&flagOutputConfig, "output-config", false, "When true, outputs a configuration file for oapi-codegen using current settings.")
	flag.StringVar(&flagConfigFile, "config", "", "A YAML config file that controls oapi-codegen behavior.")
	flag.BoolVar(&flagPrintVersion, "version", false, "When specified, print version and exit.")
	flag.StringVar(&flagPackageName, "package", "", "The package name for generated code.")
	flag.BoolVar(&flagPrintUsage, "help", false, "Show this help and exit.")
	flag.BoolVar(&flagPrintUsage, "h", false, "Same as -help.")

	flag.Parse()

	if flagPrintUsage {
		flag.Usage()
		os.Exit(0)
	}

	if flagPrintVersion {
		bi, ok := debug.ReadBuildInfo()
		if !ok {
			fmt.Fprintln(os.Stderr, "error reading build info")
			os.Exit(1)
		}
		fmt.Println(bi.Main.Path + "/cmd/oapi-codegen")
		version := bi.Main.Version
		if len(noVCSVersionOverride) > 0 {
			version = noVCSVersionOverride
		}
		fmt.Println(version)
		return
	}

	if flag.NArg() < 1 {
		errExit("Please specify a path to an OpenAPI 3.0 or 3.1 spec file\n")
	} else if flag.NArg() > 1 {
		errExit("Only one OpenAPI 3.0 or 3.1 spec file is accepted and it must be the last CLI argument\n")
	}

	var opts configuration
	if flagConfigFile != "" {
		buf, err := os.ReadFile(flagConfigFile)
		if err != nil {
			errExit("error reading config file '%s': %v\n", flagConfigFile, err)
		}
		dec := yaml.NewDecoder(bytes.NewReader(buf))
		dec.KnownFields(true)
		// An empty file decodes to io.EOF, and leaves every option unset.
		if err := dec.Decode(&opts); err != nil && !errors.Is(err, io.EOF) {
			errExit("error parsing '%s': %v\n", flagConfigFile, err)
		}
	} else {
		// Without a configuration file, generate a server, client and models.
		opts = configuration{
			Configuration: codegen.Configuration{
				Generate: codegen.GenerateOptions{
					Server: true,
					Client: true,
					Models: true,
				},
			},
		}
	}
	if flagPackageName != "" {
		opts.PackageName = flagPackageName
	}
	if opts.OutputFile == "" {
		opts.OutputFile = flagOutputFile
	}

	// Ensure default values are set if user hasn't specified some needed
	// fields.
	opts.Configuration = opts.UpdateDefaults()

	if err := detectPackageName(&opts); err != nil {
		errExit("%s\n", err)
	}

	// Now, ensure that the config options are valid.
	if err := opts.Validate(); err != nil {
		errExit("configuration error: %v\n", err)
	}

	if warnings := opts.Generate.Warnings(); len(warnings) > 0 {
		var out strings.Builder
		out.WriteString("WARNING: A number of warning(s) were returned when validating the GenerateOptions:")
		for k, v := range warnings {
			out.WriteString("\n- " + k + ": " + v)
		}

		_, _ = fmt.Fprint(os.Stderr, out.String())
	}

	if warnings := opts.Warnings(); len(warnings) > 0 {
		var out strings.Builder
		out.WriteString("WARNING: A number of cross-field configuration warning(s) were returned:")
		for k, v := range warnings {
			out.WriteString("\n- " + k + ": " + v)
		}
		out.WriteString("\n")

		_, _ = fmt.Fprint(os.Stderr, out.String())
	}

	// If the user asked to output configuration, output it to stdout and exit
	if flagOutputConfig {
		var buf bytes.Buffer
		enc := yaml.NewEncoder(&buf)
		enc.SetIndent(2)
		if err := enc.Encode(opts); err != nil {
			errExit("error YAML marshaling configuration: %v\n", err)
		}
		_ = enc.Close()
		fmt.Print(buf.String())
		return
	}

	overlayOpts := util.LoadSwaggerWithOverlayOpts{
		Path: opts.OutputOptions.Overlay.Path,
		// default to strict, but can be overridden
		Strict: true,
	}

	if opts.OutputOptions.Overlay.Strict != nil {
		overlayOpts.Strict = *opts.OutputOptions.Overlay.Strict
	}

	swagger, err := util.LoadSwaggerWithOverlay(flag.Arg(0), overlayOpts)
	if err != nil {
		errExit("error loading swagger spec in %s\n: %s\n", flag.Arg(0), err)
	}

	if len(noVCSVersionOverride) > 0 {
		opts.NoVCSVersionOverride = &noVCSVersionOverride
	}

	code, genErr := codegen.Generate(swagger, opts.Configuration)

	// Always emit any generated code to the requested destination, even when
	// generation returned an error (e.g. the formatter rejected the output).
	// Writing to the output file lets the user inspect the broken source
	// directly instead of having it interleaved with stderr.
	if code != "" {
		if opts.OutputFile != "" {
			if err := os.MkdirAll(filepath.Dir(opts.OutputFile), 0o755); err != nil {
				errExit("error unable to create directory: %s\n", err)
			}
			if err := os.WriteFile(opts.OutputFile, []byte(code), 0o644); err != nil {
				errExit("error writing generated code to file: %s\n", err)
			}
		} else {
			fmt.Print(code)
		}
	}

	if genErr != nil {
		errExit("error generating code: %s\n", genErr)
	}
}

// detectPackageName detects and sets PackageName if not already set.
func detectPackageName(cfg *configuration) error {
	if cfg.PackageName != "" {
		return nil
	}

	if cfg.OutputFile != "" {
		// Determine from the package name of the output file.
		dir := filepath.Dir(cfg.PackageName)
		cmd := exec.Command("go", "list", "-f", "{{.Name}}", dir)
		out, err := cmd.CombinedOutput()
		if err != nil {
			outStr := string(out)
			switch {
			case strings.Contains(outStr, "expected 'package', found 'EOF'"):
				// Redirecting the output to current directory which hasn't
				// written anything yet, ignore.
			case strings.HasPrefix(outStr, "no Go files in"):
				// No go files yet, ignore.
			default:
				// Unexpected failure report.
				return fmt.Errorf("detect package name for %q output: %q: %w", dir, string(out), err)
			}
		} else {
			cfg.PackageName = string(out)
			return nil
		}
	}

	// Fallback to determining from the spec file name.
	parts := strings.Split(filepath.Base(flag.Arg(0)), ".")
	cfg.PackageName = codegen.LowercaseFirstCharacter(codegen.ToCamelCase(parts[0]))

	return nil
}
