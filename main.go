package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/pflag"
	"go.uber.org/zap"
)

const pkgConfigExecName = "pkg-config"

func getArg0Path() string {
	arg0 := os.Args[0]
	if strings.Contains(arg0, "/") {
		return arg0
	}
	cwd, _ := os.Getwd()
	return filepath.Join(cwd, arg0)
}

func lookPath(arg0path string) (string, error) {
	path := os.Getenv("PATH")
	for _, dir := range filepath.SplitList(path) {
		if dir == "" {
			// Unix shell semantics: path element "" means "."
			dir = "."
		}
		dir, _ = filepath.Abs(dir)
		path := filepath.Join(dir, pkgConfigExecName)
		if arg0path == path {
			// Exclude this executable from the search path.
			continue
		} else if _, err := exec.LookPath(path); err == nil {
			return path, nil
		}
	}
	return "", errors.New("pkg-config executable not found on PATH")
}

var logger *zap.Logger

func configureLogger(logger **zap.Logger) error {
	logPath := os.Getenv("PKG_CONFIG_LOG")
	if logPath == "" {
		*logger = zap.NewNop()
		return nil
	}
	config := zap.NewProductionConfig()
	config.OutputPaths = []string{logPath}
	l, err := config.Build()
	if err != nil {
		return err
	}
	*logger = l
	return nil
}

type Flags struct {
	Cflags bool
	Libs   bool
}

func parseFlags(name string, args []string) ([]string, Flags, error) {
	var flags Flags
	flagSet := pflag.NewFlagSet(name, pflag.ContinueOnError)
	flagSet.BoolVar(&flags.Cflags, "cflags", false, "output all pre-processor and compiler flags")
	flagSet.BoolVar(&flags.Libs, "libs", false, "output all linker flags")
	if err := flagSet.Parse(args); err != nil {
		return nil, flags, err
	}
	return flagSet.Args(), flags, nil
}

func runPkgConfig(execCmd string, libs []string, flags Flags) error {
	args := make([]string, 0, len(libs)+3)
	if flags.Cflags {
		args = append(args, "--cflags")
	}
	if flags.Libs {
		args = append(args, "--libs")
	}
	args = append(args, "--")
	args = append(args, libs...)

	cmd := exec.Command(execCmd, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func realMain() int {
	if err := configureLogger(&logger); err != nil {
		panic(err)
	}

	arg0path := getArg0Path()
	logger.Info("Started pkg-config", zap.String("arg0", arg0path), zap.Strings("args", os.Args[1:]))
	pkgConfigExec, err := lookPath(getArg0Path())
	if err != nil {
		logger.Error("Could not find pkg-config executable", zap.Error(err))
		return 1
	}
	logger.Info("Found pkg-config execute", zap.String("path", pkgConfigExec))

	libs, flags, err := parseFlags(os.Args[0], os.Args[1:])
	if err != nil {
		logger.Error("Failed to parse command-line flags", zap.Error(err))
		return 1
	}

	// TODO(jsternberg): Intercept the libraries that we build,
	// build the libraries, generate the pkg-config files, then intercept
	// the path into pkg-config's search path.

	if err := runPkgConfig(pkgConfigExec, libs, flags); err != nil {
		logger.Error("Running pkg-config failed", zap.Error(err))
		return 1
	}
	return 0
}

func main() {
	os.Exit(realMain())
}
