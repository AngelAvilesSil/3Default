package conversionjobs

import (
	"errors"

	"github.com/google/uuid"
)

var ErrOutputContentSizeConflict = errors.New(
	"conversion output content size conflicts with existing hash",
)

type FinalizeSuccessInput struct {
	ConversionJobID uuid.UUID
	ContentSHA256   string
	SizeBytes       int64
	MediaType       string
}
