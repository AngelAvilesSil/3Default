package conversionjobs

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"testing"

	"github.com/AngelAvilesSil/3Default/internal/database/dbgen"
)

func TestGLBPassThroughConverterPreservesValidContent(
	t *testing.T,
) {
	content := newTestGLB([]byte{1, 2, 3, 4})

	result, err := NewGLBPassThroughConverter().Convert(
		context.Background(),
		dbgen.ProjectFile{},
		bytes.NewReader(content),
	)
	if err != nil {
		t.Fatalf("convert GLB: %v", err)
	}
	defer result.Content.Close()

	if result.MediaType != GLBMediaType {
		t.Fatalf(
			"expected media type %q, got %q",
			GLBMediaType,
			result.MediaType,
		)
	}

	got, err := io.ReadAll(result.Content)
	if err != nil {
		t.Fatalf("read converted GLB: %v", err)
	}

	if !bytes.Equal(got, content) {
		t.Fatalf(
			"expected pass-through content %x, got %x",
			content,
			got,
		)
	}
}

func TestGLBPassThroughConverterPreservesJSONOnlyGLB(
	t *testing.T,
) {
	content := newTestGLB(nil)

	result, err := NewGLBPassThroughConverter().Convert(
		context.Background(),
		dbgen.ProjectFile{},
		bytes.NewReader(content),
	)
	if err != nil {
		t.Fatalf("convert JSON-only GLB: %v", err)
	}
	defer result.Content.Close()

	got, err := io.ReadAll(result.Content)
	if err != nil {
		t.Fatalf("read converted JSON-only GLB: %v", err)
	}

	if !bytes.Equal(got, content) {
		t.Fatalf(
			"expected pass-through content %x, got %x",
			content,
			got,
		)
	}
}

func TestGLBPassThroughConverterRejectsUnsupportedContent(
	t *testing.T,
) {
	_, err := NewGLBPassThroughConverter().Convert(
		context.Background(),
		dbgen.ProjectFile{},
		bytes.NewReader([]byte("not a glb file")),
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
}

func TestGLBPassThroughConverterRejectsUnsupportedVersion(
	t *testing.T,
) {
	content := newTestGLB(nil)
	binary.LittleEndian.PutUint32(
		content[4:8],
		1,
	)

	_, err := NewGLBPassThroughConverter().Convert(
		context.Background(),
		dbgen.ProjectFile{},
		bytes.NewReader(content),
	)
	if !errors.Is(err, ErrInvalidGLB) {
		t.Fatalf(
			"expected ErrInvalidGLB, got %v",
			err,
		)
	}
}

func TestGLBPassThroughConverterDetectsDeclaredLengthMismatch(
	t *testing.T,
) {
	tests := []struct {
		name          string
		declaredDelta int32
	}{
		{
			name:          "declared too long",
			declaredDelta: 4,
		},
		{
			name:          "declared too short",
			declaredDelta: -4,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			content := newTestGLB(
				[]byte{1, 2, 3, 4},
			)

			declared := int32(
				binary.LittleEndian.Uint32(
					content[8:12],
				),
			)
			binary.LittleEndian.PutUint32(
				content[8:12],
				uint32(
					declared+
						test.declaredDelta,
				),
			)

			result, err :=
				NewGLBPassThroughConverter().Convert(
					context.Background(),
					dbgen.ProjectFile{},
					bytes.NewReader(content),
				)
			if err != nil {
				t.Fatalf(
					"create pass-through stream: %v",
					err,
				)
			}
			defer result.Content.Close()

			_, err = io.ReadAll(result.Content)
			if !errors.Is(err, ErrInvalidGLB) {
				t.Fatalf(
					"expected ErrInvalidGLB, got %v",
					err,
				)
			}
		})
	}
}

func TestGLBPassThroughConverterRejectsNonJSONFirstChunk(
	t *testing.T,
) {
	content := newTestGLB(nil)
	binary.LittleEndian.PutUint32(
		content[16:20],
		glbBINChunkType,
	)

	result, err := NewGLBPassThroughConverter().Convert(
		context.Background(),
		dbgen.ProjectFile{},
		bytes.NewReader(content),
	)
	if err != nil {
		t.Fatalf(
			"create pass-through stream: %v",
			err,
		)
	}
	defer result.Content.Close()

	_, err = io.ReadAll(result.Content)
	if !errors.Is(err, ErrInvalidGLB) {
		t.Fatalf(
			"expected ErrInvalidGLB, got %v",
			err,
		)
	}
}

func TestGLBPassThroughConverterRejectsUnalignedChunkLength(
	t *testing.T,
) {
	content := newTestGLB(nil)

	jsonLength := binary.LittleEndian.Uint32(
		content[12:16],
	)
	binary.LittleEndian.PutUint32(
		content[12:16],
		jsonLength-1,
	)

	result, err := NewGLBPassThroughConverter().Convert(
		context.Background(),
		dbgen.ProjectFile{},
		bytes.NewReader(content),
	)
	if err != nil {
		t.Fatalf(
			"create pass-through stream: %v",
			err,
		)
	}
	defer result.Content.Close()

	_, err = io.ReadAll(result.Content)
	if !errors.Is(err, ErrInvalidGLB) {
		t.Fatalf(
			"expected ErrInvalidGLB, got %v",
			err,
		)
	}
}

func TestGLBPassThroughConverterHonorsCanceledContext(
	t *testing.T,
) {
	ctx, cancel := context.WithCancel(
		context.Background(),
	)
	cancel()

	_, err := NewGLBPassThroughConverter().Convert(
		ctx,
		dbgen.ProjectFile{},
		bytes.NewReader(newTestGLB(nil)),
	)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf(
			"expected context.Canceled, got %v",
			err,
		)
	}
}

func newTestGLB(
	binaryPayload []byte,
) []byte {
	jsonPayload := padGLBChunk(
		[]byte(`{"asset":{"version":"2.0"}}`),
		' ',
	)

	content := make(
		[]byte,
		glbHeaderSize+
			glbChunkHeaderSize+
			len(jsonPayload),
	)

	binary.LittleEndian.PutUint32(
		content[0:4],
		glbMagic,
	)
	binary.LittleEndian.PutUint32(
		content[4:8],
		glbVersionTwo,
	)

	offset := glbHeaderSize
	binary.LittleEndian.PutUint32(
		content[offset:offset+4],
		uint32(len(jsonPayload)),
	)
	binary.LittleEndian.PutUint32(
		content[offset+4:offset+8],
		glbJSONChunkType,
	)
	offset += glbChunkHeaderSize

	copy(
		content[offset:offset+len(jsonPayload)],
		jsonPayload,
	)

	if binaryPayload != nil {
		binaryPayload = padGLBChunk(
			binaryPayload,
			0,
		)

		binChunk := make(
			[]byte,
			glbChunkHeaderSize+
				len(binaryPayload),
		)
		binary.LittleEndian.PutUint32(
			binChunk[0:4],
			uint32(len(binaryPayload)),
		)
		binary.LittleEndian.PutUint32(
			binChunk[4:8],
			glbBINChunkType,
		)
		copy(
			binChunk[glbChunkHeaderSize:],
			binaryPayload,
		)

		content = append(content, binChunk...)
	}

	binary.LittleEndian.PutUint32(
		content[8:12],
		uint32(len(content)),
	)

	return content
}

func padGLBChunk(
	content []byte,
	paddingByte byte,
) []byte {
	padding := (4 - len(content)%4) % 4
	if padding == 0 {
		return append([]byte(nil), content...)
	}

	result := make(
		[]byte,
		len(content)+padding,
	)
	copy(result, content)

	for i := len(content); i < len(result); i++ {
		result[i] = paddingByte
	}

	return result
}
