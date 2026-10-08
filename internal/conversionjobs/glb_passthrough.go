package conversionjobs

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"github.com/AngelAvilesSil/3Default/internal/database/dbgen"
)

var ErrInvalidGLB = errors.New(
	"GLB content is invalid",
)

const (
	GLBMediaType       = "model/gltf-binary"
	glbHeaderSize      = 12
	glbChunkHeaderSize = 8
	glbMagic           = uint32(0x46546c67)
	glbVersionTwo      = uint32(2)
	glbJSONChunkType   = uint32(0x4e4f534a)
	glbBINChunkType    = uint32(0x004e4942)
)

type GLBPassThroughConverter struct{}

func NewGLBPassThroughConverter() *GLBPassThroughConverter {
	return &GLBPassThroughConverter{}
}

func (c *GLBPassThroughConverter) Convert(
	ctx context.Context,
	_ dbgen.ProjectFile,
	source io.Reader,
) (ConversionResult, error) {
	if err := ctx.Err(); err != nil {
		return ConversionResult{}, err
	}

	var header [glbHeaderSize]byte
	if _, err := io.ReadFull(source, header[:]); err != nil {
		if errors.Is(err, io.EOF) ||
			errors.Is(err, io.ErrUnexpectedEOF) {
			return ConversionResult{},
				ErrUnsupportedConversionSource
		}

		return ConversionResult{}, fmt.Errorf(
			"read GLB header: %w",
			err,
		)
	}

	if binary.LittleEndian.Uint32(header[0:4]) != glbMagic {
		return ConversionResult{},
			ErrUnsupportedConversionSource
	}

	if binary.LittleEndian.Uint32(header[4:8]) != glbVersionTwo {
		return ConversionResult{}, fmt.Errorf(
			"%w: unsupported GLB version",
			ErrInvalidGLB,
		)
	}

	totalLength := binary.LittleEndian.Uint32(header[8:12])
	if totalLength <
		glbHeaderSize+glbChunkHeaderSize {
		return ConversionResult{}, fmt.Errorf(
			"%w: declared length cannot contain a JSON chunk",
			ErrInvalidGLB,
		)
	}

	if totalLength%4 != 0 {
		return ConversionResult{}, fmt.Errorf(
			"%w: declared length is not 4-byte aligned",
			ErrInvalidGLB,
		)
	}

	payload := &glbPayloadReader{
		ctx:       ctx,
		source:    source,
		remaining: int64(totalLength - glbHeaderSize),
	}

	return ConversionResult{
		Content: io.NopCloser(
			io.MultiReader(
				bytes.NewReader(header[:]),
				payload,
			),
		),
		MediaType: GLBMediaType,
	}, nil
}

type glbPayloadReader struct {
	ctx            context.Context
	source         io.Reader
	remaining      int64
	pendingHeader  []byte
	chunkRemaining int64
	chunkIndex     int
	sawBIN         bool
}

func (r *glbPayloadReader) Read(
	buffer []byte,
) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}

	if len(buffer) == 0 {
		return 0, nil
	}

	if len(r.pendingHeader) > 0 {
		n := copy(buffer, r.pendingHeader)
		r.pendingHeader = r.pendingHeader[n:]
		return n, nil
	}

	if r.chunkRemaining > 0 {
		readSize := int64(len(buffer))
		if readSize > r.chunkRemaining {
			readSize = r.chunkRemaining
		}

		n, err := r.source.Read(buffer[:readSize])
		r.chunkRemaining -= int64(n)
		r.remaining -= int64(n)

		if err != nil {
			if errors.Is(err, io.EOF) &&
				r.chunkRemaining > 0 {
				return n, fmt.Errorf(
					"%w: chunk content is shorter than declared length",
					ErrInvalidGLB,
				)
			}

			return n, err
		}

		return n, nil
	}

	if r.remaining == 0 {
		var extra [1]byte
		n, err := r.source.Read(extra[:])
		if n > 0 {
			return 0, fmt.Errorf(
				"%w: content is longer than declared length",
				ErrInvalidGLB,
			)
		}
		if err != nil {
			return 0, err
		}

		return 0, nil
	}

	if r.remaining < glbChunkHeaderSize {
		return 0, fmt.Errorf(
			"%w: trailing bytes cannot contain a chunk header",
			ErrInvalidGLB,
		)
	}

	var chunkHeader [glbChunkHeaderSize]byte
	if _, err := io.ReadFull(
		r.source,
		chunkHeader[:],
	); err != nil {
		return 0, fmt.Errorf(
			"%w: read chunk header: %v",
			ErrInvalidGLB,
			err,
		)
	}

	r.remaining -= glbChunkHeaderSize

	chunkLength := binary.LittleEndian.Uint32(
		chunkHeader[0:4],
	)
	chunkType := binary.LittleEndian.Uint32(
		chunkHeader[4:8],
	)

	if chunkLength%4 != 0 {
		return 0, fmt.Errorf(
			"%w: chunk length is not 4-byte aligned",
			ErrInvalidGLB,
		)
	}

	if int64(chunkLength) > r.remaining {
		return 0, fmt.Errorf(
			"%w: chunk exceeds declared total length",
			ErrInvalidGLB,
		)
	}

	if err := r.validateChunkType(chunkType); err != nil {
		return 0, err
	}

	r.chunkRemaining = int64(chunkLength)
	r.pendingHeader = chunkHeader[:]
	r.chunkIndex++

	return r.Read(buffer)
}

func (r *glbPayloadReader) validateChunkType(
	chunkType uint32,
) error {
	if r.chunkIndex == 0 {
		if chunkType != glbJSONChunkType {
			return fmt.Errorf(
				"%w: first chunk is not JSON",
				ErrInvalidGLB,
			)
		}

		return nil
	}

	if chunkType == glbJSONChunkType {
		return fmt.Errorf(
			"%w: JSON chunk appears more than once",
			ErrInvalidGLB,
		)
	}

	if chunkType == glbBINChunkType {
		if r.chunkIndex != 1 || r.sawBIN {
			return fmt.Errorf(
				"%w: BIN chunk is not the optional second chunk",
				ErrInvalidGLB,
			)
		}

		r.sawBIN = true
	}

	return nil
}
