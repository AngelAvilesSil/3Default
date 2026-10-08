package conversionjobs

import (
	"context"
	"errors"
	"io"
	"path/filepath"
	"strings"

	"github.com/AngelAvilesSil/3Default/internal/database/dbgen"
)

var (
	ErrGLBConverterRequired = errors.New(
		"GLB converter is required",
	)
	ErrSTEPConverterRequired = errors.New(
		"STEP converter is required",
	)
)

type FormatRoutingConverter struct {
	glb  Converter
	step Converter
}

func NewFormatRoutingConverter(
	glb Converter,
	step Converter,
) (*FormatRoutingConverter, error) {
	if glb == nil {
		return nil, ErrGLBConverterRequired
	}

	if step == nil {
		return nil, ErrSTEPConverterRequired
	}

	return &FormatRoutingConverter{
		glb:  glb,
		step: step,
	}, nil
}

func (c *FormatRoutingConverter) Convert(
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

	switch extension {
	case ".glb":
		return c.glb.Convert(
			ctx,
			projectFile,
			source,
		)

	case ".step", ".stp":
		return c.step.Convert(
			ctx,
			projectFile,
			source,
		)
	}

	if extension == "" &&
		projectFile.MediaType != nil &&
		strings.EqualFold(
			strings.TrimSpace(*projectFile.MediaType),
			GLBMediaType,
		) {
		return c.glb.Convert(
			ctx,
			projectFile,
			source,
		)
	}

	return ConversionResult{},
		ErrUnsupportedConversionSource
}
