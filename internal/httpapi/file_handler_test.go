package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	api "github.com/AngelAvilesSil/3Default/internal/api"
	"github.com/AngelAvilesSil/3Default/internal/database/dbgen"
	"github.com/AngelAvilesSil/3Default/internal/filestorage"
	"github.com/google/uuid"
)

type fakeProjectFileService struct {
	listCalled      bool
	listOwnerUserID uuid.UUID
	listProjectID   uuid.UUID
	files           []dbgen.ProjectFile
	listErr         error

	getCalled        bool
	getOwnerUserID   uuid.UUID
	getProjectID     uuid.UUID
	getProjectFileID uuid.UUID
	projectFile      dbgen.ProjectFile
	getErr           error
}

func (f *fakeProjectFileService) List(
	_ context.Context,
	ownerUserID uuid.UUID,
	projectID uuid.UUID,
) ([]dbgen.ProjectFile, error) {
	f.listCalled = true
	f.listOwnerUserID = ownerUserID
	f.listProjectID = projectID

	return f.files, f.listErr
}

func (f *fakeProjectFileService) Get(
	_ context.Context,
	ownerUserID uuid.UUID,
	projectID uuid.UUID,
	projectFileID uuid.UUID,
) (dbgen.ProjectFile, error) {
	f.getCalled = true
	f.getOwnerUserID = ownerUserID
	f.getProjectID = projectID
	f.getProjectFileID = projectFileID

	return f.projectFile, f.getErr
}

type fakeProjectFileUploader struct {
	called        bool
	input         filestorage.UploadInput
	projectFile   dbgen.ProjectFile
	err           error
	consumeSource bool
	sourceBytes   []byte
}

func (f *fakeProjectFileUploader) Upload(
	_ context.Context,
	input filestorage.UploadInput,
) (dbgen.ProjectFile, error) {
	f.called = true
	f.input = input

	if f.consumeSource {
		data, err := io.ReadAll(input.Source)
		if err != nil {
			return dbgen.ProjectFile{}, err
		}

		f.sourceBytes = data
	}

	return f.projectFile, f.err
}

func newProjectFileHandler(
	files ProjectFileService,
	uploads ProjectFileUploader,
) http.Handler {
	return NewHandler(
		NewServerWithVersioningAndFiles(
			fakeDatabase{},
			nil,
			nil,
			files,
			uploads,
			nil,
			nil,
			nil,
			nil,
		),
		nil,
	)
}

func newMultipartUploadRequest(
	t *testing.T,
	projectID string,
	filename string,
	content []byte,
) *http.Request {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	part, err := writer.CreateFormFile(
		"file",
		filename,
	)
	if err != nil {
		t.Fatalf("create multipart file: %v", err)
	}

	if _, err := part.Write(content); err != nil {
		t.Fatalf("write multipart file: %v", err)
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/projects/"+projectID+"/files",
		&body,
	)
	request.Header.Set(
		"Content-Type",
		writer.FormDataContentType(),
	)

	return request
}

func TestListProjectFilesRequiresAuthentication(t *testing.T) {
	files := &fakeProjectFileService{}
	handler := newProjectFileHandler(files, nil)

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/projects/"+uuid.New().String()+"/files",
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

	if files.listCalled {
		t.Fatal("expected file service not to be called")
	}
}

func TestListProjectFilesRejectsMalformedProjectID(t *testing.T) {
	files := &fakeProjectFileService{}
	handler := newProjectFileHandler(files, nil)

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/projects/not-a-uuid/files",
		nil,
	)
	request = requestWithSession(request, uuid.New())

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusBadRequest,
			response.Code,
		)
	}

	if files.listCalled {
		t.Fatal("expected file service not to be called")
	}
}

func TestListProjectFilesMapsServiceErrors(t *testing.T) {
	tests := []struct {
		name        string
		serviceErr  error
		wantStatus  int
		wantMessage string
	}{
		{
			name:        "missing project ID",
			serviceErr:  filestorage.ErrProjectIDRequired,
			wantStatus:  http.StatusBadRequest,
			wantMessage: "project ID is required",
		},
		{
			name:        "project not found",
			serviceErr:  filestorage.ErrProjectNotFound,
			wantStatus:  http.StatusNotFound,
			wantMessage: "project not found",
		},
		{
			name:        "unexpected service error",
			serviceErr:  errors.New("database unavailable"),
			wantStatus:  http.StatusInternalServerError,
			wantMessage: "unable to list project files",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			files := &fakeProjectFileService{
				listErr: test.serviceErr,
			}
			handler := newProjectFileHandler(files, nil)

			request := httptest.NewRequest(
				http.MethodGet,
				"/api/projects/"+
					uuid.New().String()+
					"/files",
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

func TestListProjectFilesReturnsFiles(t *testing.T) {
	userID := uuid.New()
	projectID := uuid.New()
	createdAt := time.Date(
		2026,
		time.September,
		28,
		22,
		0,
		0,
		0,
		time.UTC,
	)
	mediaType := "model/step"

	firstID := uuid.New()
	secondID := uuid.New()

	files := &fakeProjectFileService{
		files: []dbgen.ProjectFile{
			{
				ID:               firstID,
				ProjectID:        projectID,
				UploadedByUserID: userID,
				ContentSha256: strings.Repeat(
					"a",
					64,
				),
				OriginalFilename: "gripper.step",
				MediaType:        &mediaType,
				CreatedAt:        createdAt,
			},
			{
				ID:               secondID,
				ProjectID:        projectID,
				UploadedByUserID: userID,
				ContentSha256: strings.Repeat(
					"b",
					64,
				),
				OriginalFilename: "fixture.stp",
				CreatedAt:        createdAt.Add(time.Minute),
			},
		},
	}
	handler := newProjectFileHandler(files, nil)

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/projects/"+projectID.String()+"/files",
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

	if !files.listCalled {
		t.Fatal("expected file service to be called")
	}

	if files.listOwnerUserID != userID {
		t.Fatalf(
			"expected owner ID %s, got %s",
			userID,
			files.listOwnerUserID,
		)
	}

	if files.listProjectID != projectID {
		t.Fatalf(
			"expected project ID %s, got %s",
			projectID,
			files.listProjectID,
		)
	}

	var body []api.ProjectFileResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode file response: %v", err)
	}

	if len(body) != 2 {
		t.Fatalf("expected 2 files, got %d", len(body))
	}

	if uuid.UUID(body[0].Id) != firstID ||
		body[0].OriginalFilename != "gripper.step" ||
		body[0].MediaType == nil ||
		*body[0].MediaType != mediaType {
		t.Fatalf("unexpected first file response: %+v", body[0])
	}

	if uuid.UUID(body[1].Id) != secondID ||
		body[1].MediaType != nil {
		t.Fatalf("unexpected second file response: %+v", body[1])
	}
}

func TestListProjectFilesReturnsEmptyArray(t *testing.T) {
	handler := newProjectFileHandler(
		&fakeProjectFileService{},
		nil,
	)

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/projects/"+uuid.New().String()+"/files",
		nil,
	)
	request = requestWithSession(request, uuid.New())

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusOK,
			response.Code,
		)
	}

	var body []api.ProjectFileResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode file response: %v", err)
	}

	if body == nil {
		t.Fatal("expected empty array, got null")
	}

	if len(body) != 0 {
		t.Fatalf("expected 0 files, got %d", len(body))
	}
}

func TestGetProjectFileRequiresAuthentication(t *testing.T) {
	files := &fakeProjectFileService{}
	handler := newProjectFileHandler(files, nil)

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/projects/"+
			uuid.New().String()+
			"/files/"+
			uuid.New().String(),
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

	if files.getCalled {
		t.Fatal("expected file service not to be called")
	}
}

func TestGetProjectFileRejectsMalformedIDs(t *testing.T) {
	tests := []string{
		"/api/projects/not-a-uuid/files/" +
			uuid.New().String(),
		"/api/projects/" +
			uuid.New().String() +
			"/files/not-a-uuid",
	}

	for _, path := range tests {
		t.Run(path, func(t *testing.T) {
			files := &fakeProjectFileService{}
			handler := newProjectFileHandler(files, nil)

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

			if files.getCalled {
				t.Fatal(
					"expected file service not to be called",
				)
			}
		})
	}
}

func TestGetProjectFileMapsServiceErrors(t *testing.T) {
	tests := []struct {
		name        string
		serviceErr  error
		wantStatus  int
		wantMessage string
	}{
		{
			name:        "missing project ID",
			serviceErr:  filestorage.ErrProjectIDRequired,
			wantStatus:  http.StatusBadRequest,
			wantMessage: "project ID is required",
		},
		{
			name:        "missing project file ID",
			serviceErr:  filestorage.ErrProjectFileIDRequired,
			wantStatus:  http.StatusBadRequest,
			wantMessage: "project file ID is required",
		},
		{
			name:        "project not found",
			serviceErr:  filestorage.ErrProjectNotFound,
			wantStatus:  http.StatusNotFound,
			wantMessage: "project not found",
		},
		{
			name:        "project file not found",
			serviceErr:  filestorage.ErrProjectFileNotFound,
			wantStatus:  http.StatusNotFound,
			wantMessage: "project file not found",
		},
		{
			name:        "unexpected service error",
			serviceErr:  errors.New("database unavailable"),
			wantStatus:  http.StatusInternalServerError,
			wantMessage: "unable to get project file",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			files := &fakeProjectFileService{
				getErr: test.serviceErr,
			}
			handler := newProjectFileHandler(files, nil)

			request := httptest.NewRequest(
				http.MethodGet,
				"/api/projects/"+
					uuid.New().String()+
					"/files/"+
					uuid.New().String(),
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

func TestGetProjectFileReturnsFile(t *testing.T) {
	userID := uuid.New()
	projectID := uuid.New()
	projectFileID := uuid.New()
	createdAt := time.Date(
		2026,
		time.September,
		28,
		22,
		30,
		0,
		0,
		time.UTC,
	)
	mediaType := "model/step"

	files := &fakeProjectFileService{
		projectFile: dbgen.ProjectFile{
			ID:               projectFileID,
			ProjectID:        projectID,
			UploadedByUserID: userID,
			ContentSha256:    strings.Repeat("a", 64),
			OriginalFilename: "gripper.step",
			MediaType:        &mediaType,
			CreatedAt:        createdAt,
		},
	}
	handler := newProjectFileHandler(files, nil)

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/projects/"+
			projectID.String()+
			"/files/"+
			projectFileID.String(),
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

	if files.getOwnerUserID != userID ||
		files.getProjectID != projectID ||
		files.getProjectFileID != projectFileID {
		t.Fatalf(
			"unexpected service arguments: owner=%s project=%s file=%s",
			files.getOwnerUserID,
			files.getProjectID,
			files.getProjectFileID,
		)
	}

	var body api.ProjectFileResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode file response: %v", err)
	}

	if uuid.UUID(body.Id) != projectFileID ||
		uuid.UUID(body.ProjectId) != projectID ||
		uuid.UUID(body.UploadedByUserId) != userID ||
		body.OriginalFilename != "gripper.step" ||
		body.ContentSha256 != strings.Repeat("a", 64) ||
		body.MediaType == nil ||
		*body.MediaType != mediaType ||
		!body.CreatedAt.Equal(createdAt) {
		t.Fatalf("unexpected project file response: %+v", body)
	}
}

func TestUploadProjectFileRequiresAuthentication(t *testing.T) {
	uploader := &fakeProjectFileUploader{}
	handler := newProjectFileHandler(nil, uploader)

	request := newMultipartUploadRequest(
		t,
		uuid.New().String(),
		"part.step",
		[]byte("content"),
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

	if uploader.called {
		t.Fatal("expected uploader not to be called")
	}
}

func TestUploadProjectFileRejectsCrossOriginBrowserRequest(
	t *testing.T,
) {
	uploader := &fakeProjectFileUploader{}
	handler := newProjectFileHandler(nil, uploader)

	request := newMultipartUploadRequest(
		t,
		uuid.New().String(),
		"part.step",
		[]byte("content"),
	)
	request.Header.Set(
		"Origin",
		"https://attacker.example",
	)

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusForbidden,
			response.Code,
		)
	}

	if uploader.called {
		t.Fatal("expected uploader not to be called")
	}
}

func TestUploadProjectFileRejectsMalformedProjectID(
	t *testing.T,
) {
	uploader := &fakeProjectFileUploader{}
	handler := newProjectFileHandler(nil, uploader)

	request := newMultipartUploadRequest(
		t,
		"not-a-uuid",
		"part.step",
		[]byte("content"),
	)
	request = requestWithSession(request, uuid.New())

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusBadRequest,
			response.Code,
		)
	}

	if uploader.called {
		t.Fatal("expected uploader not to be called")
	}
}

func TestUploadProjectFileRejectsInvalidMultipartRequest(
	t *testing.T,
) {
	uploader := &fakeProjectFileUploader{}
	handler := newProjectFileHandler(nil, uploader)

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/projects/"+uuid.New().String()+"/files",
		strings.NewReader("not multipart"),
	)
	request.Header.Set(
		"Content-Type",
		"multipart/form-data",
	)
	request = requestWithSession(request, uuid.New())

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusBadRequest,
			response.Code,
		)
	}

	if uploader.called {
		t.Fatal("expected uploader not to be called")
	}
}

func TestUploadProjectFileRequiresFilePart(t *testing.T) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	if err := writer.WriteField("note", "not a file"); err != nil {
		t.Fatalf("write form field: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	uploader := &fakeProjectFileUploader{}
	handler := newProjectFileHandler(nil, uploader)

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/projects/"+uuid.New().String()+"/files",
		&body,
	)
	request.Header.Set(
		"Content-Type",
		writer.FormDataContentType(),
	)
	request = requestWithSession(request, uuid.New())

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusBadRequest,
			response.Code,
		)
	}

	if uploader.called {
		t.Fatal("expected uploader not to be called")
	}

	errorBody := decodeErrorResponse(t, response)
	if errorBody.Error != "file part is required" {
		t.Fatalf(
			"expected file-part error, got %q",
			errorBody.Error,
		)
	}
}

func TestUploadProjectFileRejectsAdditionalMultipartPart(
	t *testing.T,
) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	part, err := writer.CreateFormFile(
		"file",
		"part.step",
	)
	if err != nil {
		t.Fatalf("create file part: %v", err)
	}

	if _, err := part.Write([]byte("CAD content")); err != nil {
		t.Fatalf("write file part: %v", err)
	}

	if err := writer.WriteField(
		"unexpected",
		"value",
	); err != nil {
		t.Fatalf("write unexpected field: %v", err)
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	uploader := &fakeProjectFileUploader{
		consumeSource: true,
	}
	handler := newProjectFileHandler(nil, uploader)

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/projects/"+uuid.New().String()+"/files",
		&body,
	)
	request.Header.Set(
		"Content-Type",
		writer.FormDataContentType(),
	)
	request = requestWithSession(request, uuid.New())

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusBadRequest,
			response.Code,
		)
	}

	if !uploader.called {
		t.Fatal("expected uploader to begin consuming source")
	}

	errorBody := decodeErrorResponse(t, response)
	if errorBody.Error != "invalid multipart upload" {
		t.Fatalf(
			"expected multipart error, got %q",
			errorBody.Error,
		)
	}
}

func TestUploadProjectFileMapsServiceErrors(t *testing.T) {
	tests := []struct {
		name        string
		serviceErr  error
		wantStatus  int
		wantMessage string
	}{
		{
			name:        "missing project ID",
			serviceErr:  filestorage.ErrProjectIDRequired,
			wantStatus:  http.StatusBadRequest,
			wantMessage: "project ID is required",
		},
		{
			name:        "project not found",
			serviceErr:  filestorage.ErrProjectNotFound,
			wantStatus:  http.StatusNotFound,
			wantMessage: "project not found",
		},
		{
			name: "content metadata conflict",
			serviceErr: filestorage.
				ErrContentObjectSizeConflict,
			wantStatus:  http.StatusConflict,
			wantMessage: "stored content metadata conflict",
		},
		{
			name:        "unexpected service error",
			serviceErr:  errors.New("storage unavailable"),
			wantStatus:  http.StatusInternalServerError,
			wantMessage: "unable to upload project file",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			uploader := &fakeProjectFileUploader{
				err: test.serviceErr,
			}
			handler := newProjectFileHandler(nil, uploader)

			request := newMultipartUploadRequest(
				t,
				uuid.New().String(),
				"part.step",
				[]byte("content"),
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

			errorBody := decodeErrorResponse(t, response)
			if errorBody.Error != test.wantMessage {
				t.Fatalf(
					"expected error %q, got %q",
					test.wantMessage,
					errorBody.Error,
				)
			}
		})
	}
}

func TestUploadProjectFileForwardsMultipartFile(
	t *testing.T,
) {
	userID := uuid.New()
	projectID := uuid.New()
	projectFileID := uuid.New()
	content := []byte("authoritative CAD source")
	createdAt := time.Date(
		2026,
		time.September,
		28,
		23,
		0,
		0,
		0,
		time.UTC,
	)

	uploader := &fakeProjectFileUploader{
		consumeSource: true,
		projectFile: dbgen.ProjectFile{
			ID:               projectFileID,
			ProjectID:        projectID,
			UploadedByUserID: userID,
			ContentSha256:    strings.Repeat("a", 64),
			OriginalFilename: "gripper.step",
			CreatedAt:        createdAt,
		},
	}
	handler := newProjectFileHandler(nil, uploader)

	request := newMultipartUploadRequest(
		t,
		projectID.String(),
		"gripper.step",
		content,
	)
	request = requestWithSession(request, userID)

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusCreated,
			response.Code,
		)
	}

	if !uploader.called {
		t.Fatal("expected uploader to be called")
	}

	if uploader.input.OwnerUserID != userID {
		t.Fatalf(
			"expected owner ID %s, got %s",
			userID,
			uploader.input.OwnerUserID,
		)
	}

	if uploader.input.ProjectID != projectID {
		t.Fatalf(
			"expected project ID %s, got %s",
			projectID,
			uploader.input.ProjectID,
		)
	}

	if uploader.input.OriginalFilename != "gripper.step" {
		t.Fatalf(
			"expected filename %q, got %q",
			"gripper.step",
			uploader.input.OriginalFilename,
		)
	}

	if !bytes.Equal(uploader.sourceBytes, content) {
		t.Fatalf(
			"expected source %q, got %q",
			content,
			uploader.sourceBytes,
		)
	}

	if uploader.input.MediaType == nil ||
		*uploader.input.MediaType != "application/octet-stream" {
		t.Fatalf(
			"expected application/octet-stream media type, got %v",
			uploader.input.MediaType,
		)
	}

	var responseBody api.ProjectFileResponse
	if err := json.NewDecoder(response.Body).Decode(
		&responseBody,
	); err != nil {
		t.Fatalf("decode upload response: %v", err)
	}

	if uuid.UUID(responseBody.Id) != projectFileID ||
		uuid.UUID(responseBody.ProjectId) != projectID ||
		uuid.UUID(responseBody.UploadedByUserId) != userID ||
		responseBody.OriginalFilename != "gripper.step" {
		t.Fatalf(
			"unexpected upload response: %+v",
			responseBody,
		)
	}
}
