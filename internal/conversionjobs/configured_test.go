package conversionjobs

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/AngelAvilesSil/3Default/internal/database/dbgen"
)

func TestConfiguredConverterWithoutMayoPreservesPassThrough(
	t *testing.T,
) {
	converter, err := NewConfiguredConverter("   ")
	if err != nil {
		t.Fatalf("create converter: %v", err)
	}

	if _, ok := converter.(*GLBPassThroughConverter); !ok {
		t.Fatalf(
			"expected GLB pass-through, got %T",
			converter,
		)
	}

	// Preserve the previous content-based GLB behavior when
	// Mayo is not configured.
	content := newTestGLB(nil)

	result, err := converter.Convert(
		context.Background(),
		dbgen.ProjectFile{
			OriginalFilename: "legacy-upload.bin",
		},
		bytes.NewReader(content),
	)
	if err != nil {
		t.Fatalf("convert existing GLB: %v", err)
	}
	defer result.Content.Close()

	got, err := io.ReadAll(result.Content)
	if err != nil {
		t.Fatalf("read GLB: %v", err)
	}

	if !bytes.Equal(got, content) {
		t.Fatal("GLB pass-through changed content")
	}
}

func TestConfiguredConverterWithMayoEnablesFormatRouting(
	t *testing.T,
) {
	const executable = "/opt/mayo/AppRun"

	converter, err := NewConfiguredConverter(executable)
	if err != nil {
		t.Fatalf("create converter: %v", err)
	}

	router, ok := converter.(*FormatRoutingConverter)
	if !ok {
		t.Fatalf(
			"expected format router, got %T",
			converter,
		)
	}

	if _, ok := router.glb.(*GLBPassThroughConverter); !ok {
		t.Fatalf(
			"expected GLB pass-through delegate, got %T",
			router.glb,
		)
	}

	step, ok := router.step.(*MayoSTEPConverter)
	if !ok {
		t.Fatalf(
			"expected Mayo STEP delegate, got %T",
			router.step,
		)
	}

	if step.executable != executable {
		t.Fatalf(
			"expected executable %q, got %q",
			executable,
			step.executable,
		)
	}

	if step.headless {
		t.Fatal(
			"Mayo 0.10.0 should run directly without Xvfb",
		)
	}
}
