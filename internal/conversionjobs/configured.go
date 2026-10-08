package conversionjobs

import "strings"

// NewConfiguredConverter preserves GLB pass-through when Mayo
// is not configured and enables STEP conversion when it is.
func NewConfiguredConverter(
	mayoExecutable string,
) (Converter, error) {
	glb := NewGLBPassThroughConverter()

	if strings.TrimSpace(mayoExecutable) == "" {
		return glb, nil
	}

	step, err := NewMayoSTEPConverter(
		mayoExecutable,
		false,
	)
	if err != nil {
		return nil, err
	}

	return NewFormatRoutingConverter(
		glb,
		step,
	)
}
