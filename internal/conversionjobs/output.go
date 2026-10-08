package conversionjobs

import (
	"errors"
	"io"

	"github.com/google/uuid"
)

var (
	ErrUnsupportedConversionSource = errors.New(
		"conversion source format is unsupported",
	)
	ErrOutputContentSizeConflict = errors.New(
		"conversion output content size conflicts with existing hash",
	)
)

type ConversionResult struct {
	Content   io.ReadCloser
	MediaType string
}

type FinalizeSuccessInput struct {
	ConversionJobID uuid.UUID
	ContentSHA256   string
	SizeBytes       int64
	MediaType       string
}
