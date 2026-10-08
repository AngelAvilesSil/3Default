package conversionjobs

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/AngelAvilesSil/3Default/internal/database/dbgen"
)

type routingTestConverter struct {
	called      bool
	projectFile dbgen.ProjectFile
	source      []byte
	result      ConversionResult
	err         error
}

func (c *routingTestConverter) Convert(
	_ context.Context,
	projectFile dbgen.ProjectFile,
	source io.Reader,
) (ConversionResult, error) {
	c.called = true
	c.projectFile = projectFile

	content, err := io.ReadAll(source)
	if err != nil {
		return ConversionResult{}, err
	}

	c.source = content

	return c.result, c.err
}

func TestNewFormatRoutingConverterRejectsMissingDependencies(
	t *testing.T,
) {
	converter := &routingTestConverter{}

	tests := []struct {
		name string
		glb  Converter
		step Converter
		want error
	}{
		{
			name: "missing GLB converter",
			step: converter,
			want: ErrGLBConverterRequired,
		},
		{
			name: "missing STEP converter",
			glb:  converter,
			want: ErrSTEPConverterRequired,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			router, err := NewFormatRoutingConverter(
				test.glb,
				test.step,
			)

			if router != nil {
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

func TestFormatRoutingConverterRoutesGLBByFilename(
	t *testing.T,
) {
	tests := []string{
		"model.glb",
		"MODEL.GLB",
		" assembly.Glb ",
	}

	for _, filename := range tests {
		t.Run(filename, func(t *testing.T) {
			glb := &routingTestConverter{
				result: ConversionResult{
					MediaType: GLBMediaType,
				},
			}
			step := &routingTestConverter{}

			router, err := NewFormatRoutingConverter(
				glb,
				step,
			)
			if err != nil {
				t.Fatalf("create routing converter: %v", err)
			}

			source := []byte("glb source")

			result, err := router.Convert(
				context.Background(),
				dbgen.ProjectFile{
					OriginalFilename: filename,
				},
				bytes.NewReader(source),
			)
			if err != nil {
				t.Fatalf("route GLB: %v", err)
			}

			if !glb.called {
				t.Fatal("expected GLB converter call")
			}

			if step.called {
				t.Fatal("did not expect STEP converter call")
			}

			if !bytes.Equal(glb.source, source) {
				t.Fatalf(
					"expected source %q, got %q",
					source,
					glb.source,
				)
			}

			if result.MediaType != GLBMediaType {
				t.Fatalf(
					"expected media type %q, got %q",
					GLBMediaType,
					result.MediaType,
				)
			}
		})
	}
}

func TestFormatRoutingConverterRoutesSTEPByFilename(
	t *testing.T,
) {
	tests := []string{
		"model.step",
		"model.stp",
		"MODEL.STEP",
		"assembly.StP",
	}

	for _, filename := range tests {
		t.Run(filename, func(t *testing.T) {
			glb := &routingTestConverter{}
			step := &routingTestConverter{
				result: ConversionResult{
					MediaType: GLBMediaType,
				},
			}

			router, err := NewFormatRoutingConverter(
				glb,
				step,
			)
			if err != nil {
				t.Fatalf("create routing converter: %v", err)
			}

			source := []byte("STEP source")

			_, err = router.Convert(
				context.Background(),
				dbgen.ProjectFile{
					OriginalFilename: filename,
				},
				bytes.NewReader(source),
			)
			if err != nil {
				t.Fatalf("route STEP: %v", err)
			}

			if glb.called {
				t.Fatal("did not expect GLB converter call")
			}

			if !step.called {
				t.Fatal("expected STEP converter call")
			}

			if !bytes.Equal(step.source, source) {
				t.Fatalf(
					"expected source %q, got %q",
					source,
					step.source,
				)
			}
		})
	}
}

func TestFormatRoutingConverterUsesGLBMediaTypeWithoutExtension(
	t *testing.T,
) {
	mediaType := GLBMediaType

	glb := &routingTestConverter{}
	step := &routingTestConverter{}

	router, err := NewFormatRoutingConverter(
		glb,
		step,
	)
	if err != nil {
		t.Fatalf("create routing converter: %v", err)
	}

	_, err = router.Convert(
		context.Background(),
		dbgen.ProjectFile{
			OriginalFilename: "model",
			MediaType:        &mediaType,
		},
		bytes.NewReader([]byte("GLB source")),
	)
	if err != nil {
		t.Fatalf("route extensionless GLB: %v", err)
	}

	if !glb.called {
		t.Fatal("expected GLB converter call")
	}

	if step.called {
		t.Fatal("did not expect STEP converter call")
	}
}

func TestFormatRoutingConverterRejectsUnsupportedFormat(
	t *testing.T,
) {
	glb := &routingTestConverter{}
	step := &routingTestConverter{}

	router, err := NewFormatRoutingConverter(
		glb,
		step,
	)
	if err != nil {
		t.Fatalf("create routing converter: %v", err)
	}

	_, err = router.Convert(
		context.Background(),
		dbgen.ProjectFile{
			OriginalFilename: "model.iges",
		},
		bytes.NewReader([]byte("IGES source")),
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

	if glb.called || step.called {
		t.Fatal("expected no delegate converter call")
	}
}

func TestFormatRoutingConverterDoesNotTrustMediaTypeOverExtension(
	t *testing.T,
) {
	mediaType := GLBMediaType

	glb := &routingTestConverter{}
	step := &routingTestConverter{}

	router, err := NewFormatRoutingConverter(
		glb,
		step,
	)
	if err != nil {
		t.Fatalf("create routing converter: %v", err)
	}

	_, err = router.Convert(
		context.Background(),
		dbgen.ProjectFile{
			OriginalFilename: "model.iges",
			MediaType:        &mediaType,
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

	if glb.called || step.called {
		t.Fatal("expected no delegate converter call")
	}
}

func TestFormatRoutingConverterHonorsCanceledContext(
	t *testing.T,
) {
	glb := &routingTestConverter{}
	step := &routingTestConverter{}

	router, err := NewFormatRoutingConverter(
		glb,
		step,
	)
	if err != nil {
		t.Fatalf("create routing converter: %v", err)
	}

	ctx, cancel := context.WithCancel(
		context.Background(),
	)
	cancel()

	_, err = router.Convert(
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

	if glb.called || step.called {
		t.Fatal("expected no delegate converter call")
	}
}
