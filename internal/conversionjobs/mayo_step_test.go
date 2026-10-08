package conversionjobs

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AngelAvilesSil/3Default/internal/database/dbgen"
)

type fakeMayoCommandRunner struct {
	called bool
	name   string
	args   []string
	output []byte
	err    error
	run    func(name string, args []string) error
}

func (f *fakeMayoCommandRunner) Run(
	ctx context.Context,
	name string,
	args ...string,
) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	f.called = true
	f.name = name
	f.args = append([]string(nil), args...)

	if f.run != nil {
		if err := f.run(name, f.args); err != nil {
			return f.output, err
		}
	}

	return f.output, f.err
}

func TestNewMayoSTEPConverterRejectsInvalidDependencies(
	t *testing.T,
) {
	runner := &fakeMayoCommandRunner{}

	tests := []struct {
		name       string
		executable string
		runner     mayoCommandRunner
		want       error
	}{
		{
			name:   "missing executable",
			runner: runner,
			want:   ErrMayoExecutableRequired,
		},
		{
			name:       "blank executable",
			executable: "   ",
			runner:     runner,
			want:       ErrMayoExecutableRequired,
		},
		{
			name:       "missing runner",
			executable: "/opt/mayo/mayo-conv",
			want:       ErrMayoRunnerRequired,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			converter, err := newMayoSTEPConverter(
				test.executable,
				true,
				test.runner,
			)

			if converter != nil {
				t.Fatal("expected no converter")
			}

			if !errors.Is(err, test.want) {
				t.Fatalf(
					"expected %v, got %v",
					test.want,
					err,
				)
			}
		})
	}
}

func TestMayoSTEPConverterConvertsSTEPToValidatedGLB(
	t *testing.T,
) {
	tests := []struct {
		name          string
		filename      string
		headless      bool
		wantCommand   string
		wantArgPrefix []string
	}{
		{
			name:        "direct STEP",
			filename:    "part.step",
			wantCommand: "/opt/mayo/mayo-conv",
		},
		{
			name:        "direct STP",
			filename:    "part.stp",
			wantCommand: "/opt/mayo/mayo-conv",
		},
		{
			name:        "headless STEP",
			filename:    "PART.STEP",
			headless:    true,
			wantCommand: "xvfb-run",
			wantArgPrefix: []string{
				"--auto-servernum",
				"/opt/mayo/mayo-conv",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			sourceContent := []byte(
				"ISO-10303-21; STEP test content",
			)
			expectedGLB := newTestGLB(
				[]byte{1, 2, 3, 4},
			)

			var sourcePath string
			var outputPath string

			runner := &fakeMayoCommandRunner{
				run: func(
					_ string,
					args []string,
				) error {
					if len(args) < 3 {
						t.Fatalf(
							"unexpected Mayo arguments: %v",
							args,
						)
					}

					sourcePath = args[len(args)-3]
					if args[len(args)-2] != "--export" {
						t.Fatalf(
							"expected --export argument, got %v",
							args,
						)
					}
					outputPath = args[len(args)-1]

					gotSource, err := os.ReadFile(
						sourcePath,
					)
					if err != nil {
						t.Fatalf(
							"read temporary STEP source: %v",
							err,
						)
					}

					if !bytes.Equal(
						gotSource,
						sourceContent,
					) {
						t.Fatalf(
							"expected source %q, got %q",
							sourceContent,
							gotSource,
						)
					}

					return os.WriteFile(
						outputPath,
						expectedGLB,
						0o600,
					)
				},
			}

			converter, err := newMayoSTEPConverter(
				"/opt/mayo/mayo-conv",
				test.headless,
				runner,
			)
			if err != nil {
				t.Fatalf(
					"create Mayo converter: %v",
					err,
				)
			}

			result, err := converter.Convert(
				context.Background(),
				dbgen.ProjectFile{
					OriginalFilename: test.filename,
				},
				bytes.NewReader(sourceContent),
			)
			if err != nil {
				t.Fatalf(
					"convert STEP: %v",
					err,
				)
			}

			if !runner.called {
				t.Fatal("expected Mayo command call")
			}

			if runner.name != test.wantCommand {
				t.Fatalf(
					"expected command %q, got %q",
					test.wantCommand,
					runner.name,
				)
			}

			if len(test.wantArgPrefix) > 0 {
				if len(runner.args) <
					len(test.wantArgPrefix) {
					t.Fatalf(
						"missing command prefix: %v",
						runner.args,
					)
				}

				for index, want := range test.wantArgPrefix {
					if runner.args[index] != want {
						t.Fatalf(
							"expected argument %d to be %q, got %q",
							index,
							want,
							runner.args[index],
						)
					}
				}
			}

			if strings.ToLower(
				filepath.Ext(sourcePath),
			) != strings.ToLower(
				filepath.Ext(test.filename),
			) {
				t.Fatalf(
					"expected temporary source extension to match %q, got %q",
					test.filename,
					sourcePath,
				)
			}

			if filepath.Ext(outputPath) != ".glb" {
				t.Fatalf(
					"expected GLB output path, got %q",
					outputPath,
				)
			}

			if result.MediaType != GLBMediaType {
				t.Fatalf(
					"expected media type %q, got %q",
					GLBMediaType,
					result.MediaType,
				)
			}

			gotGLB, err := io.ReadAll(result.Content)
			if err != nil {
				_ = result.Content.Close()
				t.Fatalf(
					"read converted GLB: %v",
					err,
				)
			}

			if !bytes.Equal(gotGLB, expectedGLB) {
				_ = result.Content.Close()
				t.Fatalf(
					"converted GLB does not match expected content",
				)
			}

			tempDir := filepath.Dir(sourcePath)

			if _, err := os.Stat(tempDir); err != nil {
				_ = result.Content.Close()
				t.Fatalf(
					"expected temporary directory before close: %v",
					err,
				)
			}

			if err := result.Content.Close(); err != nil {
				t.Fatalf(
					"close converted GLB: %v",
					err,
				)
			}

			if _, err := os.Stat(tempDir); !errors.Is(
				err,
				os.ErrNotExist,
			) {
				t.Fatalf(
					"expected temporary directory removal, got %v",
					err,
				)
			}
		})
	}
}

func TestMayoSTEPConverterRejectsUnsupportedFormat(
	t *testing.T,
) {
	runner := &fakeMayoCommandRunner{}

	converter, err := newMayoSTEPConverter(
		"/opt/mayo/mayo-conv",
		true,
		runner,
	)
	if err != nil {
		t.Fatalf("create Mayo converter: %v", err)
	}

	_, err = converter.Convert(
		context.Background(),
		dbgen.ProjectFile{
			OriginalFilename: "model.iges",
		},
		bytes.NewReader([]byte("source")),
	)
	if !errors.Is(
		err,
		ErrUnsupportedConversionSource,
	) {
		t.Fatalf(
			"expected ErrUnsupportedConversionSource, got %v",
			err,
		)
	}

	if runner.called {
		t.Fatal("did not expect Mayo command call")
	}
}

func TestMayoSTEPConverterHonorsCanceledContext(
	t *testing.T,
) {
	runner := &fakeMayoCommandRunner{}

	converter, err := newMayoSTEPConverter(
		"/opt/mayo/mayo-conv",
		true,
		runner,
	)
	if err != nil {
		t.Fatalf("create Mayo converter: %v", err)
	}

	ctx, cancel := context.WithCancel(
		context.Background(),
	)
	cancel()

	_, err = converter.Convert(
		ctx,
		dbgen.ProjectFile{
			OriginalFilename: "model.step",
		},
		bytes.NewReader([]byte("source")),
	)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf(
			"expected context.Canceled, got %v",
			err,
		)
	}

	if runner.called {
		t.Fatal("did not expect Mayo command call")
	}
}

func TestMayoSTEPConverterMapsCommandFailure(
	t *testing.T,
) {
	var tempDir string

	runner := &fakeMayoCommandRunner{
		output: []byte("Mayo reported a conversion error"),
		err:    errors.New("exit status 1"),
		run: func(
			_ string,
			args []string,
		) error {
			tempDir = filepath.Dir(
				args[len(args)-3],
			)
			return nil
		},
	}

	converter, err := newMayoSTEPConverter(
		"/opt/mayo/mayo-conv",
		true,
		runner,
	)
	if err != nil {
		t.Fatalf("create Mayo converter: %v", err)
	}

	_, err = converter.Convert(
		context.Background(),
		dbgen.ProjectFile{
			OriginalFilename: "model.step",
		},
		bytes.NewReader([]byte("source")),
	)
	if !errors.Is(err, ErrMayoConversionFailed) {
		t.Fatalf(
			"expected ErrMayoConversionFailed, got %v",
			err,
		)
	}

	if !strings.Contains(
		err.Error(),
		"Mayo reported a conversion error",
	) {
		t.Fatalf(
			"expected Mayo diagnostic output, got %v",
			err,
		)
	}

	if _, statErr := os.Stat(tempDir); !errors.Is(
		statErr,
		os.ErrNotExist,
	) {
		t.Fatalf(
			"expected failed conversion cleanup, got %v",
			statErr,
		)
	}
}

func TestMayoSTEPConverterRejectsMissingOutput(
	t *testing.T,
) {
	runner := &fakeMayoCommandRunner{}

	converter, err := newMayoSTEPConverter(
		"/opt/mayo/mayo-conv",
		false,
		runner,
	)
	if err != nil {
		t.Fatalf("create Mayo converter: %v", err)
	}

	_, err = converter.Convert(
		context.Background(),
		dbgen.ProjectFile{
			OriginalFilename: "model.step",
		},
		bytes.NewReader([]byte("source")),
	)
	if !errors.Is(err, ErrMayoConversionFailed) {
		t.Fatalf(
			"expected ErrMayoConversionFailed, got %v",
			err,
		)
	}
}

func TestMayoSTEPConverterRejectsNonGLBOutput(
	t *testing.T,
) {
	runner := &fakeMayoCommandRunner{
		run: func(
			_ string,
			args []string,
		) error {
			outputPath := args[len(args)-1]

			return os.WriteFile(
				outputPath,
				[]byte("not a GLB"),
				0o600,
			)
		},
	}

	converter, err := newMayoSTEPConverter(
		"/opt/mayo/mayo-conv",
		false,
		runner,
	)
	if err != nil {
		t.Fatalf("create Mayo converter: %v", err)
	}

	_, err = converter.Convert(
		context.Background(),
		dbgen.ProjectFile{
			OriginalFilename: "model.step",
		},
		bytes.NewReader([]byte("source")),
	)
	if !errors.Is(err, ErrMayoConversionFailed) {
		t.Fatalf(
			"expected ErrMayoConversionFailed, got %v",
			err,
		)
	}
}
