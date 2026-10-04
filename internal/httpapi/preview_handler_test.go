package httpapi

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/AngelAvilesSil/3Default/internal/auth"
	"github.com/AngelAvilesSil/3Default/internal/conversionjobs"
	"github.com/google/uuid"
)

type fakeConversionPreviewService struct {
	called        bool
	ownerUserID   uuid.UUID
	projectID     uuid.UUID
	projectFileID uuid.UUID
	result        conversionjobs.PreviewResult
	err           error
}

func (f *fakeConversionPreviewService) GetLatest(
	_ context.Context,
	ownerUserID uuid.UUID,
	projectID uuid.UUID,
	projectFileID uuid.UUID,
) (conversionjobs.PreviewResult, error) {
	f.called = true
	f.ownerUserID = ownerUserID
	f.projectID = projectID
	f.projectFileID = projectFileID

	return f.result, f.err
}

func newProjectFilePreviewHandler(
	previews ConversionPreviewService,
) http.Handler {
	return NewHandler(
		NewServerWithConversionPreviews(
			fakeDatabase{},
			nil,
			nil,
			nil,
			nil,
			nil,
			nil,
			previews,
			nil,
			nil,
			nil,
			nil,
		),
		nil,
	)
}

func newProjectFilePreviewHandlerWithResolver(
	previews ConversionPreviewService,
	resolver SessionResolver,
) http.Handler {
	return NewHandler(
		NewServerWithConversionPreviews(
			fakeDatabase{},
			nil,
			nil,
			nil,
			nil,
			nil,
			nil,
			previews,
			nil,
			nil,
			nil,
			nil,
		),
		resolver,
	)
}

func TestGetProjectFilePreviewRequiresAuthentication(
	t *testing.T,
) {
	previews := &fakeConversionPreviewService{}
	handler := newProjectFilePreviewHandler(previews)

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/projects/"+
			uuid.New().String()+
			"/files/"+
			uuid.New().String()+
			"/preview",
		nil,
	)

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusUnauthorized,
			response.Code,
		)
	}

	if previews.called {
		t.Fatal(
			"expected preview service not to be called",
		)
	}

	body := decodeErrorResponse(t, response)
	if body.Error != "authentication required" {
		t.Fatalf(
			"expected authentication error, got %q",
			body.Error,
		)
	}
}

func TestGetProjectFilePreviewRejectsSessionResolutionFailure(
	t *testing.T,
) {
	previews := &fakeConversionPreviewService{}
	resolver := &fakeSessionResolver{
		err: errors.New("session database unavailable"),
	}

	handler := newProjectFilePreviewHandlerWithResolver(
		previews,
		resolver,
	)

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/projects/"+
			uuid.New().String()+
			"/files/"+
			uuid.New().String()+
			"/preview",
		nil,
	)
	request.AddCookie(&http.Cookie{
		Name:  sessionCookieName,
		Value: "session-token",
	})

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusInternalServerError,
			response.Code,
		)
	}

	if !resolver.called {
		t.Fatal("expected session resolver call")
	}

	if previews.called {
		t.Fatal(
			"expected preview service not to be called",
		)
	}

	body := decodeErrorResponse(t, response)
	if body.Error != "unable to authenticate request" {
		t.Fatalf(
			"expected authentication failure, got %q",
			body.Error,
		)
	}
}

func TestGetProjectFilePreviewRequiresDependency(
	t *testing.T,
) {
	handler := newProjectFilePreviewHandler(nil)

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/projects/"+
			uuid.New().String()+
			"/files/"+
			uuid.New().String()+
			"/preview",
		nil,
	)
	request = requestWithSession(
		request,
		uuid.New(),
	)

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusInternalServerError,
			response.Code,
		)
	}

	body := decodeErrorResponse(t, response)
	if body.Error != "unable to get project file preview" {
		t.Fatalf(
			"expected preview dependency error, got %q",
			body.Error,
		)
	}
}

func TestGetProjectFilePreviewRejectsMalformedIDs(
	t *testing.T,
) {
	projectID := uuid.New()
	projectFileID := uuid.New()

	tests := []string{
		"/api/projects/not-a-uuid/files/" +
			projectFileID.String() +
			"/preview",
		"/api/projects/" +
			projectID.String() +
			"/files/not-a-uuid/preview",
	}

	for _, path := range tests {
		t.Run(path, func(t *testing.T) {
			previews := &fakeConversionPreviewService{}
			handler := newProjectFilePreviewHandler(
				previews,
			)

			request := httptest.NewRequest(
				http.MethodGet,
				path,
				nil,
			)
			request = requestWithSession(
				request,
				uuid.New(),
			)

			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)

			if response.Code != http.StatusBadRequest {
				t.Fatalf(
					"expected status %d, got %d",
					http.StatusBadRequest,
					response.Code,
				)
			}

			if previews.called {
				t.Fatal(
					"expected preview service not to be called",
				)
			}
		})
	}
}

func TestGetProjectFilePreviewMapsServiceErrors(
	t *testing.T,
) {
	tests := []struct {
		name        string
		serviceErr  error
		wantStatus  int
		wantMessage string
	}{
		{
			name:        "missing project ID",
			serviceErr:  conversionjobs.ErrProjectIDRequired,
			wantStatus:  http.StatusBadRequest,
			wantMessage: "project ID is required",
		},
		{
			name: "missing project file ID",
			serviceErr: conversionjobs.
				ErrProjectFileIDRequired,
			wantStatus:  http.StatusBadRequest,
			wantMessage: "project file ID is required",
		},
		{
			name:        "project not found",
			serviceErr:  conversionjobs.ErrProjectNotFound,
			wantStatus:  http.StatusNotFound,
			wantMessage: "project not found",
		},
		{
			name: "project file not found",
			serviceErr: conversionjobs.
				ErrProjectFileNotFound,
			wantStatus:  http.StatusNotFound,
			wantMessage: "project file not found",
		},
		{
			name:        "preview not found",
			serviceErr:  conversionjobs.ErrPreviewNotFound,
			wantStatus:  http.StatusNotFound,
			wantMessage: "preview not found",
		},
		{
			name: "unsupported preview media type",
			serviceErr: conversionjobs.
				ErrPreviewMediaTypeUnsupported,
			wantStatus:  http.StatusInternalServerError,
			wantMessage: "unable to get project file preview",
		},
		{
			name:        "unexpected error",
			serviceErr:  errors.New("storage unavailable"),
			wantStatus:  http.StatusInternalServerError,
			wantMessage: "unable to get project file preview",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			previews := &fakeConversionPreviewService{
				err: test.serviceErr,
			}
			handler := newProjectFilePreviewHandler(
				previews,
			)

			request := httptest.NewRequest(
				http.MethodGet,
				"/api/projects/"+
					uuid.New().String()+
					"/files/"+
					uuid.New().String()+
					"/preview",
				nil,
			)
			request = requestWithSession(
				request,
				uuid.New(),
			)

			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)

			if response.Code != test.wantStatus {
				t.Fatalf(
					"expected status %d, got %d",
					test.wantStatus,
					response.Code,
				)
			}

			body := decodeErrorResponse(t, response)
			if body.Error != test.wantMessage {
				t.Fatalf(
					"expected error %q, got %q",
					test.wantMessage,
					body.Error,
				)
			}
		})
	}
}

func TestGetProjectFilePreviewRejectsNilContent(
	t *testing.T,
) {
	previews := &fakeConversionPreviewService{
		result: conversionjobs.PreviewResult{},
	}
	handler := newProjectFilePreviewHandler(previews)

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/projects/"+
			uuid.New().String()+
			"/files/"+
			uuid.New().String()+
			"/preview",
		nil,
	)
	request = requestWithSession(
		request,
		uuid.New(),
	)

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusInternalServerError,
			response.Code,
		)
	}

	body := decodeErrorResponse(t, response)
	if body.Error != "unable to get project file preview" {
		t.Fatalf(
			"expected preview error, got %q",
			body.Error,
		)
	}
}

func TestGetProjectFilePreviewStreamsGLB(
	t *testing.T,
) {
	userID := uuid.New()
	projectID := uuid.New()
	projectFileID := uuid.New()
	expectedContent := []byte(
		"derived GLB preview bytes",
	)

	content := &trackingReadCloser{
		Reader: bytes.NewReader(expectedContent),
	}

	previews := &fakeConversionPreviewService{
		result: conversionjobs.PreviewResult{
			Content: content,
		},
	}
	handler := newProjectFilePreviewHandler(previews)

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/projects/"+
			projectID.String()+
			"/files/"+
			projectFileID.String()+
			"/preview",
		nil,
	)
	request = requestWithSession(request, userID)

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusOK,
			response.Code,
		)
	}

	if !previews.called {
		t.Fatal("expected preview service call")
	}

	if previews.ownerUserID != userID ||
		previews.projectID != projectID ||
		previews.projectFileID != projectFileID {
		t.Fatalf(
			"unexpected preview arguments: owner=%s project=%s file=%s",
			previews.ownerUserID,
			previews.projectID,
			previews.projectFileID,
		)
	}

	if response.Header().Get("Content-Type") !=
		conversionjobs.GLBMediaType {
		t.Fatalf(
			"expected %q content type, got %q",
			conversionjobs.GLBMediaType,
			response.Header().Get("Content-Type"),
		)
	}

	if response.Header().Get("Content-Disposition") != "" {
		t.Fatalf(
			"expected no Content-Disposition header, got %q",
			response.Header().Get("Content-Disposition"),
		)
	}

	if !bytes.Equal(
		response.Body.Bytes(),
		expectedContent,
	) {
		t.Fatalf(
			"expected preview content %q, got %q",
			expectedContent,
			response.Body.Bytes(),
		)
	}

	if !content.closed {
		t.Fatal(
			"expected streamed preview content to be closed",
		)
	}
}

func TestGetProjectFilePreviewMiddlewareResolvesSession(
	t *testing.T,
) {
	userID := uuid.New()
	projectID := uuid.New()
	projectFileID := uuid.New()

	content := &trackingReadCloser{
		Reader: bytes.NewReader([]byte("GLB")),
	}

	previews := &fakeConversionPreviewService{
		result: conversionjobs.PreviewResult{
			Content: content,
		},
	}
	resolver := &fakeSessionResolver{
		session: auth.Session{
			ID:     uuid.New(),
			UserID: userID,
		},
	}

	handler := newProjectFilePreviewHandlerWithResolver(
		previews,
		resolver,
	)

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/projects/"+
			projectID.String()+
			"/files/"+
			projectFileID.String()+
			"/preview",
		nil,
	)
	request.AddCookie(&http.Cookie{
		Name:  sessionCookieName,
		Value: "session-token",
	})

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusOK,
			response.Code,
		)
	}

	if !resolver.called {
		t.Fatal("expected session resolver call")
	}

	if !previews.called {
		t.Fatal("expected preview service call")
	}

	if previews.ownerUserID != userID {
		t.Fatalf(
			"expected owner user ID %s, got %s",
			userID,
			previews.ownerUserID,
		)
	}
}
