package conversionjobs

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/AngelAvilesSil/3Default/internal/database/dbgen"
)

var (
	ErrMayoExecutableRequired = errors.New(
		"Mayo executable is required",
	)
	ErrMayoRunnerRequired = errors.New(
		"Mayo command runner is required",
	)
	ErrMayoConversionFailed = errors.New(
		"Mayo conversion failed",
	)
)

type mayoCommandRunner interface {
	Run(
		ctx context.Context,
		name string,
		args ...string,
	) ([]byte, error)
}

type execMayoCommandRunner struct{}

func (execMayoCommandRunner) Run(
	ctx context.Context,
	name string,
	args ...string,
) ([]byte, error) {
	command := exec.CommandContext(
		ctx,
		name,
		args...,
	)
	configureMayoCommand(command)

	return command.CombinedOutput()
}

type MayoSTEPConverter struct {
	executable string
	headless   bool
	runner     mayoCommandRunner
}

func NewMayoSTEPConverter(
	executable string,
	headless bool,
) (*MayoSTEPConverter, error) {
	return newMayoSTEPConverter(
		executable,
		headless,
		execMayoCommandRunner{},
	)
}

func newMayoSTEPConverter(
	executable string,
	headless bool,
	runner mayoCommandRunner,
) (*MayoSTEPConverter, error) {
	executable = strings.TrimSpace(executable)
	if executable == "" {
		return nil, ErrMayoExecutableRequired
	}

	if runner == nil {
		return nil, ErrMayoRunnerRequired
	}

	return &MayoSTEPConverter{
		executable: executable,
		headless:   headless,
		runner:     runner,
	}, nil
}

func (c *MayoSTEPConverter) Convert(
	ctx context.Context,
	projectFile dbgen.ProjectFile,
	source io.Reader,
) (ConversionResult, error) {
	if err := ctx.Err(); err != nil {
		return ConversionResult{}, err
	}

	extension := strings.ToLower(
		filepath.Ext(
			strings.TrimSpace(
				projectFile.OriginalFilename,
			),
		),
	)

	if extension != ".step" &&
		extension != ".stp" {
		return ConversionResult{},
			ErrUnsupportedConversionSource
	}

	tempDir, err := os.MkdirTemp(
		"",
		"3default-step-conversion-*",
	)
	if err != nil {
		return ConversionResult{}, fmt.Errorf(
			"create STEP conversion temporary directory: %w",
			err,
		)
	}

	cleanup := true
	defer func() {
		if cleanup {
			_ = os.RemoveAll(tempDir)
		}
	}()

	sourcePath := filepath.Join(
		tempDir,
		"source"+extension,
	)
	outputPath := filepath.Join(
		tempDir,
		"preview.glb",
	)

	if err := writeMayoSource(
		ctx,
		sourcePath,
		source,
	); err != nil {
		return ConversionResult{}, err
	}

	commandName, commandArgs := c.command(
		sourcePath,
		outputPath,
	)

	commandOutput, err := c.runner.Run(
		ctx,
		commandName,
		commandArgs...,
	)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ConversionResult{}, ctxErr
		}

		message := strings.TrimSpace(
			string(commandOutput),
		)
		if message == "" {
			return ConversionResult{}, fmt.Errorf(
				"%w: %v",
				ErrMayoConversionFailed,
				err,
			)
		}

		return ConversionResult{}, fmt.Errorf(
			"%w: %v: %s",
			ErrMayoConversionFailed,
			err,
			message,
		)
	}

	outputFile, err := os.Open(outputPath)
	if err != nil {
		return ConversionResult{}, fmt.Errorf(
			"%w: open generated GLB: %v",
			ErrMayoConversionFailed,
			err,
		)
	}

	glbResult, err :=
		NewGLBPassThroughConverter().Convert(
			ctx,
			projectFile,
			outputFile,
		)
	if err != nil {
		_ = outputFile.Close()

		return ConversionResult{}, fmt.Errorf(
			"%w: validate generated GLB: %v",
			ErrMayoConversionFailed,
			err,
		)
	}

	glbResult.Content = &mayoResultReadCloser{
		content: glbResult.Content,
		file:    outputFile,
		tempDir: tempDir,
	}

	cleanup = false

	return glbResult, nil
}

func (c *MayoSTEPConverter) command(
	sourcePath string,
	outputPath string,
) (string, []string) {
	conversionArgs := []string{
		sourcePath,
		"--export",
		outputPath,
	}

	if !c.headless {
		return c.executable, conversionArgs
	}

	return "xvfb-run", append(
		[]string{
			"--auto-servernum",
			c.executable,
		},
		conversionArgs...,
	)
}

func writeMayoSource(
	ctx context.Context,
	path string,
	source io.Reader,
) error {
	file, err := os.OpenFile(
		path,
		os.O_WRONLY|os.O_CREATE|os.O_EXCL,
		0o600,
	)
	if err != nil {
		return fmt.Errorf(
			"create STEP conversion source file: %w",
			err,
		)
	}

	_, copyErr := io.Copy(
		file,
		&mayoContextReader{
			ctx:    ctx,
			source: source,
		},
	)
	closeErr := file.Close()

	if copyErr != nil {
		return fmt.Errorf(
			"write STEP conversion source file: %w",
			copyErr,
		)
	}

	if closeErr != nil {
		return fmt.Errorf(
			"close STEP conversion source file: %w",
			closeErr,
		)
	}

	return nil
}

type mayoContextReader struct {
	ctx    context.Context
	source io.Reader
}

func (r *mayoContextReader) Read(
	buffer []byte,
) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}

	return r.source.Read(buffer)
}

type mayoResultReadCloser struct {
	content io.ReadCloser
	file    *os.File
	tempDir string
}

func (r *mayoResultReadCloser) Read(
	buffer []byte,
) (int, error) {
	return r.content.Read(buffer)
}

func (r *mayoResultReadCloser) Close() error {
	return errors.Join(
		r.content.Close(),
		r.file.Close(),
		os.RemoveAll(r.tempDir),
	)
}
