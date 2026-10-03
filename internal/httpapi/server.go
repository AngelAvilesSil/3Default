package httpapi

import (
	"context"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"strings"
	"time"
	"uuid"

	api "github.com/AngelAvilesSil/3Default/internal/api"
	"github.com/AngelAvilesSil/3Default/internal/auth"
	"github.com/AngelAvilesSil/3Default/internal/conversionjobs"
	"github.com/AngelAvilesSil/3Default/internal/database/dbgen"
	"github.com/AngelAvilesSil/3Default/internal/filestorage"
	"github.com/AngelAvilesSil/3Default/internal/projects"
	"github.com/AngelAvilesSil/3Default/internal/versioning"
	googleuuid "github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type DatabasePinger interface {
	Ping(context.Context) error
}

type ProjectService interface {
	Create(
		ctx context.Context,
		input projects.CreateInput,
	) (dbgen.Project, error)
	List(
		ctx context.Context,
		ownerUserID googleuuid.UUID,
	) ([]dbgen.Project, error)
	Get(
		ctx context.Context,
		ownerUserID googleuuid.UUID,
		projectID googleuuid.UUID,
	) (dbgen.Project, error)
	Update(
		ctx context.Context,
		input projects.UpdateInput,
	) (dbgen.Project, error)
}

type VersioningService interface {
	CreateBranch(
		ctx context.Context,
		input versioning.CreateBranchInput,
	) (dbgen.ProjectBranch, error)
	RenameBranch(
		ctx context.Context,
		input versioning.RenameBranchInput,
	) (dbgen.ProjectBranch, error)
	DeleteBranch(
		ctx context.Context,
		input versioning.DeleteBranchInput,
	) error
	CreateRevision(
		ctx context.Context,
		input versioning.CreateRevisionInput,
	) (dbgen.ProjectRevision, error)
	ListBranches(
		ctx context.Context,
		ownerUserID googleuuid.UUID,
		projectID googleuuid.UUID,
	) ([]dbgen.ProjectBranch, error)
	ListBranchHistory(
		ctx context.Context,
		ownerUserID googleuuid.UUID,
		projectID googleuuid.UUID,
		branchID googleuuid.UUID,
	) ([]dbgen.ProjectRevision, error)
	GetRevision(
		ctx context.Context,
		ownerUserID googleuuid.UUID,
		projectID googleuuid.UUID,
		revisionID googleuuid.UUID,
	) (dbgen.ProjectRevision, error)
	ListRevisionFiles(
		ctx context.Context,
		ownerUserID googleuuid.UUID,
		projectID googleuuid.UUID,
		revisionID googleuuid.UUID,
	) ([]dbgen.ProjectFile, error)
}

type ProjectFileService interface {
	List(
		ctx context.Context,
		ownerUserID googleuuid.UUID,
		projectID googleuuid.UUID,
	) ([]dbgen.ProjectFile, error)
	Get(
		ctx context.Context,
		ownerUserID googleuuid.UUID,
		projectID googleuuid.UUID,
		projectFileID googleuuid.UUID,
	) (dbgen.ProjectFile, error)
}

type ProjectFileUploader interface {
	Upload(
		ctx context.Context,
		input filestorage.UploadInput,
	) (dbgen.ProjectFile, error)
}

type ProjectFileDownloader interface {
	Download(
		ctx context.Context,
		ownerUserID googleuuid.UUID,
		projectID googleuuid.UUID,
		projectFileID googleuuid.UUID,
	) (filestorage.DownloadResult, error)
}

type ConversionJobService interface {
	Create(
		ctx context.Context,
		input conversionjobs.CreateInput,
	) (dbgen.ConversionJob, error)
	Get(
		ctx context.Context,
		ownerUserID googleuuid.UUID,
		projectID googleuuid.UUID,
		conversionJobID googleuuid.UUID,
	) (dbgen.ConversionJob, error)
	ListForProjectFile(
		ctx context.Context,
		ownerUserID googleuuid.UUID,
		projectID googleuuid.UUID,
		projectFileID googleuuid.UUID,
	) ([]dbgen.ConversionJob, error)
}

type UserRegistrar interface {
	Register(
		ctx context.Context,
		input auth.RegisterInput,
	) (dbgen.User, error)
}

type UserAuthenticator interface {
	Login(
		ctx context.Context,
		input auth.LoginInput,
	) (auth.LoginResult, error)
}

type SessionRevoker interface {
	Revoke(
		ctx context.Context,
		token string,
	) error
}

type CurrentUserReader interface {
	GetUserByID(
		ctx context.Context,
		id googleuuid.UUID,
	) (dbgen.User, error)
}

type Server struct {
	database           DatabasePinger
	projects           ProjectService
	versioning         VersioningService
	projectFiles       ProjectFileService
	fileUploads        ProjectFileUploader
	fileDownloads      ProjectFileDownloader
	conversionJobs     ConversionJobService
	registrations      UserRegistrar
	authentication     UserAuthenticator
	sessionRevocations SessionRevoker
	currentUsers       CurrentUserReader
}

func NewServer(
	database DatabasePinger,
	projects ProjectService,
	registrations UserRegistrar,
	authentication UserAuthenticator,
	sessionRevocations SessionRevoker,
	currentUsers CurrentUserReader,
) *Server {
	return &Server{
		database:           database,
		projects:           projects,
		registrations:      registrations,
		authentication:     authentication,
		sessionRevocations: sessionRevocations,
		currentUsers:       currentUsers,
	}
}

func NewServerWithVersioning(
	database DatabasePinger,
	projects ProjectService,
	versioningService VersioningService,
	registrations UserRegistrar,
	authentication UserAuthenticator,
	sessionRevocations SessionRevoker,
	currentUsers CurrentUserReader,
) *Server {
	server := NewServer(
		database,
		projects,
		registrations,
		authentication,
		sessionRevocations,
		currentUsers,
	)
	server.versioning = versioningService

	return server
}

func NewServerWithVersioningAndFiles(
	database DatabasePinger,
	projects ProjectService,
	versioningService VersioningService,
	projectFiles ProjectFileService,
	fileUploads ProjectFileUploader,
	fileDownloads ProjectFileDownloader,
	registrations UserRegistrar,
	authentication UserAuthenticator,
	sessionRevocations SessionRevoker,
	currentUsers CurrentUserReader,
) *Server {
	server := NewServerWithVersioning(
		database,
		projects,
		versioningService,
		registrations,
		authentication,
		sessionRevocations,
		currentUsers,
	)

	server.projectFiles = projectFiles
	server.fileUploads = fileUploads
	server.fileDownloads = fileDownloads

	return server
}

func NewServerWithConversionJobs(
	database DatabasePinger,
	projects ProjectService,
	versioningService VersioningService,
	projectFiles ProjectFileService,
	fileUploads ProjectFileUploader,
	fileDownloads ProjectFileDownloader,
	conversionJobs ConversionJobService,
	registrations UserRegistrar,
	authentication UserAuthenticator,
	sessionRevocations SessionRevoker,
	currentUsers CurrentUserReader,
) *Server {
	server := NewServerWithVersioningAndFiles(
		database,
		projects,
		versioningService,
		projectFiles,
		fileUploads,
		fileDownloads,
		registrations,
		authentication,
		sessionRevocations,
		currentUsers,
	)

	server.conversionJobs = conversionJobs

	return server
}

func (s *Server) GetHealth(
	_ context.Context,
	_ api.GetHealthRequestObject,
) (api.GetHealthResponseObject, error) {
	return api.GetHealth200JSONResponse{Status: api.Ok}, nil
}

func (s *Server) GetReady(
	ctx context.Context,
	_ api.GetReadyRequestObject,
) (api.GetReadyResponseObject, error) {
	pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	if err := s.database.Ping(pingCtx); err != nil {
		return api.GetReady503JSONResponse{Status: api.Unavailable}, nil
	}

	return api.GetReady200JSONResponse{Status: api.Ready}, nil
}

func (s *Server) LoginUser(
	ctx context.Context,
	request api.LoginUserRequestObject,
) (api.LoginUserResponseObject, error) {
	if request.Body == nil {
		return api.LoginUser400JSONResponse{
			Error: "request body is required",
		}, nil
	}

	result, err := s.authentication.Login(
		ctx,
		auth.LoginInput{
			Email:    request.Body.Email,
			Password: request.Body.Password,
		},
	)
	if err != nil {
		if errors.Is(err, auth.ErrInvalidCredentials) {
			return api.LoginUser401JSONResponse{
				Error: "invalid email or password",
			}, nil
		}

		if errors.Is(err, auth.ErrLoginRateLimited) {
			return api.LoginUser429JSONResponse{
				Error: "too many login attempts",
			}, nil
		}

		return api.LoginUser500JSONResponse{
			Error: "unable to log in",
		}, nil
	}

	sessionCookie := NewSessionCookie(
		result.Session.Token,
		result.Session.Session.ExpiresAt,
	).String()

	return api.LoginUser200JSONResponse{
		Body: api.UserResponse{
			Id:          uuid.UUID(result.User.ID),
			Email:       result.User.Email,
			DisplayName: result.User.DisplayName,
			CreatedAt:   result.User.CreatedAt,
			UpdatedAt:   result.User.UpdatedAt,
		},
		Headers: api.LoginUser200ResponseHeaders{
			SetCookie: &sessionCookie,
		},
	}, nil
}

func (s *Server) LogoutUser(
	ctx context.Context,
	_ api.LogoutUserRequestObject,
) (api.LogoutUserResponseObject, error) {
	expiredSessionCookie := NewExpiredSessionCookie().String()

	success := api.LogoutUser204Response{
		Headers: api.LogoutUser204ResponseHeaders{
			SetCookie: &expiredSessionCookie,
		},
	}

	token, ok := SessionTokenFromContext(ctx)
	if !ok {
		return success, nil
	}

	if err := s.sessionRevocations.Revoke(
		ctx,
		token,
	); err != nil {
		if errors.Is(err, auth.ErrInvalidSessionToken) {
			return success, nil
		}

		return api.LogoutUser500JSONResponse{
			Error: "unable to log out",
		}, nil
	}

	return success, nil
}

func (s *Server) GetCurrentUser(
	ctx context.Context,
	_ api.GetCurrentUserRequestObject,
) (api.GetCurrentUserResponseObject, error) {
	if err := SessionResolutionError(ctx); err != nil {
		return api.GetCurrentUser500JSONResponse{
			Error: "unable to authenticate request",
		}, nil
	}

	session, ok := SessionFromContext(ctx)
	if !ok {
		return api.GetCurrentUser401JSONResponse{
			Error: "authentication required",
		}, nil
	}

	user, err := s.currentUsers.GetUserByID(
		ctx,
		session.UserID,
	)
	if err != nil {
		return api.GetCurrentUser500JSONResponse{
			Error: "unable to get current user",
		}, nil
	}

	return api.GetCurrentUser200JSONResponse{
		Id:          uuid.UUID(user.ID),
		Email:       user.Email,
		DisplayName: user.DisplayName,
		CreatedAt:   user.CreatedAt,
		UpdatedAt:   user.UpdatedAt,
	}, nil
}

func (s *Server) RegisterUser(
	ctx context.Context,
	request api.RegisterUserRequestObject,
) (api.RegisterUserResponseObject, error) {
	if request.Body == nil {
		return api.RegisterUser400JSONResponse{
			Error: "request body is required",
		}, nil
	}

	user, err := s.registrations.Register(
		ctx,
		auth.RegisterInput{
			Email:       request.Body.Email,
			DisplayName: request.Body.DisplayName,
			Password:    request.Body.Password,
		},
	)
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrEmailRequired):
			return api.RegisterUser400JSONResponse{
				Error: "email is required",
			}, nil

		case errors.Is(err, auth.ErrDisplayNameRequired):
			return api.RegisterUser400JSONResponse{
				Error: "display name is required",
			}, nil

		case errors.Is(err, auth.ErrPasswordInvalidUTF8):
			return api.RegisterUser400JSONResponse{
				Error: "password contains invalid UTF-8",
			}, nil

		case errors.Is(err, auth.ErrPasswordTooShort):
			return api.RegisterUser400JSONResponse{
				Error: "password must contain at least 15 characters",
			}, nil

		case errors.Is(err, auth.ErrPasswordTooLong):
			return api.RegisterUser400JSONResponse{
				Error: "password must contain at most 128 characters",
			}, nil

		case errors.Is(err, auth.ErrPasswordBlocked):
			return api.RegisterUser400JSONResponse{
				Error: "password is too common, expected, or compromised",
			}, nil

		case errors.Is(err, auth.ErrEmailAlreadyRegistered):
			return api.RegisterUser409JSONResponse{
				Error: "email is already registered",
			}, nil

		default:
			return api.RegisterUser500JSONResponse{
				Error: "unable to register user",
			}, nil
		}
	}

	return api.RegisterUser201JSONResponse{
		Id:          uuid.UUID(user.ID),
		Email:       user.Email,
		DisplayName: user.DisplayName,
		CreatedAt:   user.CreatedAt,
		UpdatedAt:   user.UpdatedAt,
	}, nil
}

func (s *Server) ListProjects(
	ctx context.Context,
	_ api.ListProjectsRequestObject,
) (api.ListProjectsResponseObject, error) {
	if err := SessionResolutionError(ctx); err != nil {
		return api.ListProjects500JSONResponse{
			Error: "unable to authenticate request",
		}, nil
	}

	session, ok := SessionFromContext(ctx)
	if !ok {
		return api.ListProjects401JSONResponse{
			Error: "authentication required",
		}, nil
	}

	projectRows, err := s.projects.List(
		ctx,
		session.UserID,
	)
	if err != nil {
		return api.ListProjects500JSONResponse{
			Error: "unable to list projects",
		}, nil
	}

	response := make(
		api.ListProjects200JSONResponse,
		0,
		len(projectRows),
	)

	for _, project := range projectRows {
		response = append(
			response,
			api.ProjectResponse{
				Id:          uuid.UUID(project.ID),
				Name:        project.Name,
				Description: project.Description,
				Visibility: api.ProjectResponseVisibility(
					project.Visibility,
				),
				CreatedAt: project.CreatedAt,
				UpdatedAt: project.UpdatedAt,
			},
		)
	}

	return response, nil
}

func (s *Server) GetProject(
	ctx context.Context,
	request api.GetProjectRequestObject,
) (api.GetProjectResponseObject, error) {
	if err := SessionResolutionError(ctx); err != nil {
		return api.GetProject500JSONResponse{
			Error: "unable to authenticate request",
		}, nil
	}

	session, ok := SessionFromContext(ctx)
	if !ok {
		return api.GetProject401JSONResponse{
			Error: "authentication required",
		}, nil
	}

	project, err := s.projects.Get(
		ctx,
		session.UserID,
		googleuuid.UUID(request.ProjectId),
	)
	if err != nil {
		if errors.Is(err, projects.ErrProjectIDRequired) {
			return api.GetProject400JSONResponse{
				Error: "project ID is required",
			}, nil
		}

		if errors.Is(err, projects.ErrProjectNotFound) {
			return api.GetProject404JSONResponse{
				Error: "project not found",
			}, nil
		}

		return api.GetProject500JSONResponse{
			Error: "unable to get project",
		}, nil
	}

	return api.GetProject200JSONResponse{
		Id:          uuid.UUID(project.ID),
		Name:        project.Name,
		Description: project.Description,
		Visibility:  api.ProjectResponseVisibility(project.Visibility),
		CreatedAt:   project.CreatedAt,
		UpdatedAt:   project.UpdatedAt,
	}, nil
}

func (s *Server) UpdateProject(
	ctx context.Context,
	request api.UpdateProjectRequestObject,
) (api.UpdateProjectResponseObject, error) {
	if err := SessionResolutionError(ctx); err != nil {
		return api.UpdateProject500JSONResponse{
			Error: "unable to authenticate request",
		}, nil
	}

	session, ok := SessionFromContext(ctx)
	if !ok {
		return api.UpdateProject401JSONResponse{
			Error: "authentication required",
		}, nil
	}

	if request.Body == nil {
		return api.UpdateProject400JSONResponse{
			Error: "request body is required",
		}, nil
	}

	nameSet := request.Body.Name.IsSpecified()

	var name *string
	if nameSet && !request.Body.Name.IsNull() {
		value := request.Body.Name.MustGet()
		name = &value
	}

	descriptionSet := request.Body.Description.IsSpecified()

	var description *string
	if descriptionSet && !request.Body.Description.IsNull() {
		value := request.Body.Description.MustGet()
		description = &value
	}

	project, err := s.projects.Update(
		ctx,
		projects.UpdateInput{
			OwnerUserID:    session.UserID,
			ProjectID:      googleuuid.UUID(request.ProjectId),
			NameSet:        nameSet,
			Name:           name,
			DescriptionSet: descriptionSet,
			Description:    description,
		},
	)
	if err != nil {
		switch {
		case errors.Is(err, projects.ErrProjectIDRequired):
			return api.UpdateProject400JSONResponse{
				Error: "project ID is required",
			}, nil

		case errors.Is(err, projects.ErrNoMetadataChanges):
			return api.UpdateProject400JSONResponse{
				Error: "project metadata changes are required",
			}, nil

		case errors.Is(err, projects.ErrNameRequired):
			return api.UpdateProject400JSONResponse{
				Error: "project name is required",
			}, nil

		case errors.Is(err, projects.ErrProjectNotFound):
			return api.UpdateProject404JSONResponse{
				Error: "project not found",
			}, nil

		default:
			return api.UpdateProject500JSONResponse{
				Error: "unable to update project",
			}, nil
		}
	}

	return api.UpdateProject200JSONResponse{
		Id:          uuid.UUID(project.ID),
		Name:        project.Name,
		Description: project.Description,
		Visibility:  api.ProjectResponseVisibility(project.Visibility),
		CreatedAt:   project.CreatedAt,
		UpdatedAt:   project.UpdatedAt,
	}, nil
}

func (s *Server) CreateProject(
	ctx context.Context,
	request api.CreateProjectRequestObject,
) (api.CreateProjectResponseObject, error) {
	if err := SessionResolutionError(ctx); err != nil {
		return api.CreateProject500JSONResponse{
			Error: "unable to authenticate request",
		}, nil
	}

	session, ok := SessionFromContext(ctx)
	if !ok {
		return api.CreateProject401JSONResponse{
			Error: "authentication required",
		}, nil
	}

	if request.Body == nil {
		return api.CreateProject400JSONResponse{
			Error: "request body is required",
		}, nil
	}

	project, err := s.projects.Create(
		ctx,
		projects.CreateInput{
			OwnerUserID: session.UserID,
			Name:        request.Body.Name,
			Description: request.Body.Description,
		},
	)
	if err != nil {
		if errors.Is(err, projects.ErrNameRequired) {
			return api.CreateProject400JSONResponse{
				Error: "project name is required",
			}, nil
		}

		return api.CreateProject500JSONResponse{
			Error: "unable to create project",
		}, nil
	}

	return api.CreateProject201JSONResponse{
		Id:          uuid.UUID(project.ID),
		Name:        project.Name,
		Description: project.Description,
		Visibility:  api.ProjectResponseVisibility(project.Visibility),
		CreatedAt:   project.CreatedAt,
		UpdatedAt:   project.UpdatedAt,
	}, nil
}

func (s *Server) CreateProjectBranch(
	ctx context.Context,
	request api.CreateProjectBranchRequestObject,
) (api.CreateProjectBranchResponseObject, error) {
	if err := SessionResolutionError(ctx); err != nil {
		return api.CreateProjectBranch500JSONResponse{
			Error: "unable to authenticate request",
		}, nil
	}

	session, ok := SessionFromContext(ctx)
	if !ok {
		return api.CreateProjectBranch401JSONResponse{
			Error: "authentication required",
		}, nil
	}

	if s.versioning == nil {
		return api.CreateProjectBranch500JSONResponse{
			Error: "unable to create project branch",
		}, nil
	}

	if request.Body == nil {
		return api.CreateProjectBranch400JSONResponse{
			Error: "request body is required",
		}, nil
	}

	if !request.Body.HeadRevisionId.IsSpecified() {
		return api.CreateProjectBranch400JSONResponse{
			Error: "head revision ID is required",
		}, nil
	}

	var headRevisionID *googleuuid.UUID
	if !request.Body.HeadRevisionId.IsNull() {
		value := request.Body.HeadRevisionId.MustGet()
		converted := googleuuid.UUID(value)
		headRevisionID = &converted
	}

	branch, err := s.versioning.CreateBranch(
		ctx,
		versioning.CreateBranchInput{
			OwnerUserID:    session.UserID,
			ProjectID:      googleuuid.UUID(request.ProjectId),
			Name:           request.Body.Name,
			HeadRevisionID: headRevisionID,
		},
	)
	if err != nil {
		switch {
		case errors.Is(err, versioning.ErrProjectIDRequired):
			return api.CreateProjectBranch400JSONResponse{
				Error: "project ID is required",
			}, nil

		case errors.Is(err, versioning.ErrBranchNameRequired):
			return api.CreateProjectBranch400JSONResponse{
				Error: "branch name is required",
			}, nil

		case errors.Is(err, versioning.ErrHeadRevisionIDInvalid):
			return api.CreateProjectBranch400JSONResponse{
				Error: "branch head revision ID is invalid",
			}, nil

		case errors.Is(err, versioning.ErrProjectNotFound):
			return api.CreateProjectBranch404JSONResponse{
				Error: "project not found",
			}, nil

		case errors.Is(err, versioning.ErrRevisionNotFound):
			return api.CreateProjectBranch404JSONResponse{
				Error: "head revision not found",
			}, nil

		case errors.Is(err, versioning.ErrBranchNameConflict):
			return api.CreateProjectBranch409JSONResponse{
				Error: "branch name already exists",
			}, nil

		default:
			return api.CreateProjectBranch500JSONResponse{
				Error: "unable to create project branch",
			}, nil
		}
	}

	return api.CreateProjectBranch201JSONResponse{
		Id:             uuid.UUID(branch.ID),
		ProjectId:      uuid.UUID(branch.ProjectID),
		Name:           branch.Name,
		HeadRevisionId: apiUUIDFromPGUUID(branch.HeadRevisionID),
		CreatedAt:      branch.CreatedAt,
		UpdatedAt:      branch.UpdatedAt,
	}, nil
}

func (s *Server) RenameProjectBranch(
	ctx context.Context,
	request api.RenameProjectBranchRequestObject,
) (api.RenameProjectBranchResponseObject, error) {
	if err := SessionResolutionError(ctx); err != nil {
		return api.RenameProjectBranch500JSONResponse{
			Error: "unable to authenticate request",
		}, nil
	}

	session, ok := SessionFromContext(ctx)
	if !ok {
		return api.RenameProjectBranch401JSONResponse{
			Error: "authentication required",
		}, nil
	}

	if s.versioning == nil {
		return api.RenameProjectBranch500JSONResponse{
			Error: "unable to rename project branch",
		}, nil
	}

	if request.Body == nil {
		return api.RenameProjectBranch400JSONResponse{
			Error: "request body is required",
		}, nil
	}

	branch, err := s.versioning.RenameBranch(
		ctx,
		versioning.RenameBranchInput{
			OwnerUserID: session.UserID,
			ProjectID:   googleuuid.UUID(request.ProjectId),
			BranchID:    googleuuid.UUID(request.BranchId),
			Name:        request.Body.Name,
		},
	)
	if err != nil {
		switch {
		case errors.Is(err, versioning.ErrProjectIDRequired):
			return api.RenameProjectBranch400JSONResponse{
				Error: "project ID is required",
			}, nil

		case errors.Is(err, versioning.ErrBranchIDRequired):
			return api.RenameProjectBranch400JSONResponse{
				Error: "branch ID is required",
			}, nil

		case errors.Is(err, versioning.ErrBranchNameRequired):
			return api.RenameProjectBranch400JSONResponse{
				Error: "branch name is required",
			}, nil

		case errors.Is(err, versioning.ErrProjectNotFound):
			return api.RenameProjectBranch404JSONResponse{
				Error: "project not found",
			}, nil

		case errors.Is(err, versioning.ErrBranchNotFound):
			return api.RenameProjectBranch404JSONResponse{
				Error: "branch not found",
			}, nil

		case errors.Is(err, versioning.ErrBranchNameConflict):
			return api.RenameProjectBranch409JSONResponse{
				Error: "branch name already exists",
			}, nil

		default:
			return api.RenameProjectBranch500JSONResponse{
				Error: "unable to rename project branch",
			}, nil
		}
	}

	return api.RenameProjectBranch200JSONResponse{
		Id:             uuid.UUID(branch.ID),
		ProjectId:      uuid.UUID(branch.ProjectID),
		Name:           branch.Name,
		HeadRevisionId: apiUUIDFromPGUUID(branch.HeadRevisionID),
		CreatedAt:      branch.CreatedAt,
		UpdatedAt:      branch.UpdatedAt,
	}, nil
}

func (s *Server) DeleteProjectBranch(
	ctx context.Context,
	request api.DeleteProjectBranchRequestObject,
) (api.DeleteProjectBranchResponseObject, error) {
	if err := SessionResolutionError(ctx); err != nil {
		return api.DeleteProjectBranch500JSONResponse{
			Error: "unable to authenticate request",
		}, nil
	}

	session, ok := SessionFromContext(ctx)
	if !ok {
		return api.DeleteProjectBranch401JSONResponse{
			Error: "authentication required",
		}, nil
	}

	if s.versioning == nil {
		return api.DeleteProjectBranch500JSONResponse{
			Error: "unable to delete project branch",
		}, nil
	}

	err := s.versioning.DeleteBranch(
		ctx,
		versioning.DeleteBranchInput{
			OwnerUserID: session.UserID,
			ProjectID:   googleuuid.UUID(request.ProjectId),
			BranchID:    googleuuid.UUID(request.BranchId),
		},
	)
	if err != nil {
		switch {
		case errors.Is(err, versioning.ErrProjectIDRequired):
			return api.DeleteProjectBranch400JSONResponse{
				Error: "project ID is required",
			}, nil

		case errors.Is(err, versioning.ErrBranchIDRequired):
			return api.DeleteProjectBranch400JSONResponse{
				Error: "branch ID is required",
			}, nil

		case errors.Is(err, versioning.ErrProjectNotFound):
			return api.DeleteProjectBranch404JSONResponse{
				Error: "project not found",
			}, nil

		case errors.Is(err, versioning.ErrBranchNotFound):
			return api.DeleteProjectBranch404JSONResponse{
				Error: "branch not found",
			}, nil

		default:
			return api.DeleteProjectBranch500JSONResponse{
				Error: "unable to delete project branch",
			}, nil
		}
	}

	return api.DeleteProjectBranch204Response{}, nil
}

func (s *Server) ListProjectBranches(
	ctx context.Context,
	request api.ListProjectBranchesRequestObject,
) (api.ListProjectBranchesResponseObject, error) {
	if err := SessionResolutionError(ctx); err != nil {
		return api.ListProjectBranches500JSONResponse{
			Error: "unable to authenticate request",
		}, nil
	}

	session, ok := SessionFromContext(ctx)
	if !ok {
		return api.ListProjectBranches401JSONResponse{
			Error: "authentication required",
		}, nil
	}

	if s.versioning == nil {
		return api.ListProjectBranches500JSONResponse{
			Error: "unable to list project branches",
		}, nil
	}

	branches, err := s.versioning.ListBranches(
		ctx,
		session.UserID,
		googleuuid.UUID(request.ProjectId),
	)
	if err != nil {
		switch {
		case errors.Is(err, versioning.ErrProjectIDRequired):
			return api.ListProjectBranches400JSONResponse{
				Error: "project ID is required",
			}, nil

		case errors.Is(err, versioning.ErrProjectNotFound):
			return api.ListProjectBranches404JSONResponse{
				Error: "project not found",
			}, nil

		default:
			return api.ListProjectBranches500JSONResponse{
				Error: "unable to list project branches",
			}, nil
		}
	}

	response := make(
		api.ListProjectBranches200JSONResponse,
		0,
		len(branches),
	)

	for _, branch := range branches {
		response = append(
			response,
			api.ProjectBranchResponse{
				Id:             uuid.UUID(branch.ID),
				ProjectId:      uuid.UUID(branch.ProjectID),
				Name:           branch.Name,
				HeadRevisionId: apiUUIDFromPGUUID(branch.HeadRevisionID),
				CreatedAt:      branch.CreatedAt,
				UpdatedAt:      branch.UpdatedAt,
			},
		)
	}

	return response, nil
}

func (s *Server) ListProjectBranchHistory(
	ctx context.Context,
	request api.ListProjectBranchHistoryRequestObject,
) (api.ListProjectBranchHistoryResponseObject, error) {
	if err := SessionResolutionError(ctx); err != nil {
		return api.ListProjectBranchHistory500JSONResponse{
			Error: "unable to authenticate request",
		}, nil
	}

	session, ok := SessionFromContext(ctx)
	if !ok {
		return api.ListProjectBranchHistory401JSONResponse{
			Error: "authentication required",
		}, nil
	}

	if s.versioning == nil {
		return api.ListProjectBranchHistory500JSONResponse{
			Error: "unable to list project branch history",
		}, nil
	}

	revisions, err := s.versioning.ListBranchHistory(
		ctx,
		session.UserID,
		googleuuid.UUID(request.ProjectId),
		googleuuid.UUID(request.BranchId),
	)
	if err != nil {
		switch {
		case errors.Is(err, versioning.ErrProjectIDRequired):
			return api.ListProjectBranchHistory400JSONResponse{
				Error: "project ID is required",
			}, nil

		case errors.Is(err, versioning.ErrBranchIDRequired):
			return api.ListProjectBranchHistory400JSONResponse{
				Error: "branch ID is required",
			}, nil

		case errors.Is(err, versioning.ErrProjectNotFound):
			return api.ListProjectBranchHistory404JSONResponse{
				Error: "project not found",
			}, nil

		case errors.Is(err, versioning.ErrBranchNotFound):
			return api.ListProjectBranchHistory404JSONResponse{
				Error: "branch not found",
			}, nil

		default:
			return api.ListProjectBranchHistory500JSONResponse{
				Error: "unable to list project branch history",
			}, nil
		}
	}

	response := make(
		api.ListProjectBranchHistory200JSONResponse,
		0,
		len(revisions),
	)

	for _, revision := range revisions {
		response = append(
			response,
			api.ProjectRevisionResponse{
				Id:                    uuid.UUID(revision.ID),
				ProjectId:             uuid.UUID(revision.ProjectID),
				AuthorUserId:          uuid.UUID(revision.AuthorUserID),
				Message:               revision.Message,
				ParentRevisionId:      apiUUIDFromPGUUID(revision.ParentRevisionID),
				MergeParentRevisionId: apiUUIDFromPGUUID(revision.MergeParentRevisionID),
				CreatedAt:             revision.CreatedAt,
			},
		)
	}

	return response, nil
}

func (s *Server) CreateProjectRevision(
	ctx context.Context,
	request api.CreateProjectRevisionRequestObject,
) (api.CreateProjectRevisionResponseObject, error) {
	if err := SessionResolutionError(ctx); err != nil {
		return api.CreateProjectRevision500JSONResponse{
			Error: "unable to authenticate request",
		}, nil
	}

	session, ok := SessionFromContext(ctx)
	if !ok {
		return api.CreateProjectRevision401JSONResponse{
			Error: "authentication required",
		}, nil
	}

	if s.versioning == nil {
		return api.CreateProjectRevision500JSONResponse{
			Error: "unable to create project revision",
		}, nil
	}

	if request.Body == nil {
		return api.CreateProjectRevision400JSONResponse{
			Error: "request body is required",
		}, nil
	}

	if request.Body.ProjectFileIds == nil {
		return api.CreateProjectRevision400JSONResponse{
			Error: "project file IDs are required",
		}, nil
	}

	if !request.Body.ExpectedHeadRevisionId.IsSpecified() {
		return api.CreateProjectRevision400JSONResponse{
			Error: "expected head revision ID is required",
		}, nil
	}

	projectFileIDs := make(
		[]googleuuid.UUID,
		len(request.Body.ProjectFileIds),
	)
	for index, projectFileID := range request.Body.ProjectFileIds {
		projectFileIDs[index] = googleuuid.UUID(projectFileID)
	}

	var expectedHeadRevisionID *googleuuid.UUID
	if !request.Body.ExpectedHeadRevisionId.IsNull() {
		value := request.Body.ExpectedHeadRevisionId.MustGet()
		converted := googleuuid.UUID(value)
		expectedHeadRevisionID = &converted
	}

	var mergeParentRevisionID *googleuuid.UUID
	if request.Body.MergeParentRevisionId != nil {
		converted := googleuuid.UUID(
			*request.Body.MergeParentRevisionId,
		)
		mergeParentRevisionID = &converted
	}

	revision, err := s.versioning.CreateRevision(
		ctx,
		versioning.CreateRevisionInput{
			OwnerUserID:            session.UserID,
			ProjectID:              googleuuid.UUID(request.ProjectId),
			BranchID:               googleuuid.UUID(request.BranchId),
			Message:                request.Body.Message,
			ExpectedHeadRevisionID: expectedHeadRevisionID,
			MergeParentRevisionID:  mergeParentRevisionID,
			ProjectFileIDs:         projectFileIDs,
		},
	)
	if err != nil {
		switch {
		case errors.Is(err, versioning.ErrProjectIDRequired):
			return api.CreateProjectRevision400JSONResponse{
				Error: "project ID is required",
			}, nil

		case errors.Is(err, versioning.ErrBranchIDRequired):
			return api.CreateProjectRevision400JSONResponse{
				Error: "branch ID is required",
			}, nil

		case errors.Is(err, versioning.ErrMessageRequired):
			return api.CreateProjectRevision400JSONResponse{
				Error: "revision message is required",
			}, nil

		case errors.Is(
			err,
			versioning.ErrExpectedHeadRevisionIDInvalid,
		):
			return api.CreateProjectRevision400JSONResponse{
				Error: "expected head revision ID is invalid",
			}, nil

		case errors.Is(
			err,
			versioning.ErrMergeParentRevisionIDInvalid,
		):
			return api.CreateProjectRevision400JSONResponse{
				Error: "merge parent revision ID is invalid",
			}, nil

		case errors.Is(err, versioning.ErrMergeRequiresHead):
			return api.CreateProjectRevision400JSONResponse{
				Error: "merge revision requires an expected branch head",
			}, nil

		case errors.Is(
			err,
			versioning.ErrRevisionParentsMustDiffer,
		):
			return api.CreateProjectRevision400JSONResponse{
				Error: "revision parents must differ",
			}, nil

		case errors.Is(err, versioning.ErrProjectFileIDInvalid):
			return api.CreateProjectRevision400JSONResponse{
				Error: "project file ID is invalid",
			}, nil

		case errors.Is(err, versioning.ErrDuplicateProjectFileID):
			return api.CreateProjectRevision400JSONResponse{
				Error: "duplicate project file ID",
			}, nil

		case errors.Is(err, versioning.ErrProjectNotFound):
			return api.CreateProjectRevision404JSONResponse{
				Error: "project not found",
			}, nil

		case errors.Is(err, versioning.ErrBranchNotFound):
			return api.CreateProjectRevision404JSONResponse{
				Error: "branch not found",
			}, nil

		case errors.Is(err, versioning.ErrProjectFileNotFound):
			return api.CreateProjectRevision404JSONResponse{
				Error: "project file not found",
			}, nil

		case errors.Is(err, versioning.ErrBranchHeadConflict):
			return api.CreateProjectRevision409JSONResponse{
				Error: "branch head changed",
			}, nil

		default:
			return api.CreateProjectRevision500JSONResponse{
				Error: "unable to create project revision",
			}, nil
		}
	}

	return api.CreateProjectRevision201JSONResponse{
		Id:                    uuid.UUID(revision.ID),
		ProjectId:             uuid.UUID(revision.ProjectID),
		AuthorUserId:          uuid.UUID(revision.AuthorUserID),
		Message:               revision.Message,
		ParentRevisionId:      apiUUIDFromPGUUID(revision.ParentRevisionID),
		MergeParentRevisionId: apiUUIDFromPGUUID(revision.MergeParentRevisionID),
		CreatedAt:             revision.CreatedAt,
	}, nil
}

func (s *Server) GetProjectRevision(
	ctx context.Context,
	request api.GetProjectRevisionRequestObject,
) (api.GetProjectRevisionResponseObject, error) {
	if err := SessionResolutionError(ctx); err != nil {
		return api.GetProjectRevision500JSONResponse{
			Error: "unable to authenticate request",
		}, nil
	}

	session, ok := SessionFromContext(ctx)
	if !ok {
		return api.GetProjectRevision401JSONResponse{
			Error: "authentication required",
		}, nil
	}

	if s.versioning == nil {
		return api.GetProjectRevision500JSONResponse{
			Error: "unable to get project revision",
		}, nil
	}

	revision, err := s.versioning.GetRevision(
		ctx,
		session.UserID,
		googleuuid.UUID(request.ProjectId),
		googleuuid.UUID(request.RevisionId),
	)
	if err != nil {
		switch {
		case errors.Is(err, versioning.ErrProjectIDRequired):
			return api.GetProjectRevision400JSONResponse{
				Error: "project ID is required",
			}, nil

		case errors.Is(err, versioning.ErrRevisionIDRequired):
			return api.GetProjectRevision400JSONResponse{
				Error: "revision ID is required",
			}, nil

		case errors.Is(err, versioning.ErrProjectNotFound):
			return api.GetProjectRevision404JSONResponse{
				Error: "project not found",
			}, nil

		case errors.Is(err, versioning.ErrRevisionNotFound):
			return api.GetProjectRevision404JSONResponse{
				Error: "revision not found",
			}, nil

		default:
			return api.GetProjectRevision500JSONResponse{
				Error: "unable to get project revision",
			}, nil
		}
	}

	return api.GetProjectRevision200JSONResponse{
		Id:                    uuid.UUID(revision.ID),
		ProjectId:             uuid.UUID(revision.ProjectID),
		AuthorUserId:          uuid.UUID(revision.AuthorUserID),
		Message:               revision.Message,
		ParentRevisionId:      apiUUIDFromPGUUID(revision.ParentRevisionID),
		MergeParentRevisionId: apiUUIDFromPGUUID(revision.MergeParentRevisionID),
		CreatedAt:             revision.CreatedAt,
	}, nil
}

func (s *Server) ListProjectRevisionFiles(
	ctx context.Context,
	request api.ListProjectRevisionFilesRequestObject,
) (api.ListProjectRevisionFilesResponseObject, error) {
	if err := SessionResolutionError(ctx); err != nil {
		return api.ListProjectRevisionFiles500JSONResponse{
			Error: "unable to authenticate request",
		}, nil
	}

	session, ok := SessionFromContext(ctx)
	if !ok {
		return api.ListProjectRevisionFiles401JSONResponse{
			Error: "authentication required",
		}, nil
	}

	if s.versioning == nil {
		return api.ListProjectRevisionFiles500JSONResponse{
			Error: "unable to list project revision files",
		}, nil
	}

	projectFiles, err := s.versioning.ListRevisionFiles(
		ctx,
		session.UserID,
		googleuuid.UUID(request.ProjectId),
		googleuuid.UUID(request.RevisionId),
	)
	if err != nil {
		switch {
		case errors.Is(err, versioning.ErrProjectIDRequired):
			return api.ListProjectRevisionFiles400JSONResponse{
				Error: "project ID is required",
			}, nil

		case errors.Is(err, versioning.ErrRevisionIDRequired):
			return api.ListProjectRevisionFiles400JSONResponse{
				Error: "revision ID is required",
			}, nil

		case errors.Is(err, versioning.ErrProjectNotFound):
			return api.ListProjectRevisionFiles404JSONResponse{
				Error: "project not found",
			}, nil

		case errors.Is(err, versioning.ErrRevisionNotFound):
			return api.ListProjectRevisionFiles404JSONResponse{
				Error: "revision not found",
			}, nil

		default:
			return api.ListProjectRevisionFiles500JSONResponse{
				Error: "unable to list project revision files",
			}, nil
		}
	}

	response := make(
		api.ListProjectRevisionFiles200JSONResponse,
		0,
		len(projectFiles),
	)

	for _, projectFile := range projectFiles {
		response = append(
			response,
			projectFileResponse(projectFile),
		)
	}

	return response, nil
}

func (s *Server) ListProjectFiles(
	ctx context.Context,
	request api.ListProjectFilesRequestObject,
) (api.ListProjectFilesResponseObject, error) {
	if err := SessionResolutionError(ctx); err != nil {
		return api.ListProjectFiles500JSONResponse{
			Error: "unable to authenticate request",
		}, nil
	}

	session, ok := SessionFromContext(ctx)
	if !ok {
		return api.ListProjectFiles401JSONResponse{
			Error: "authentication required",
		}, nil
	}

	if s.projectFiles == nil {
		return api.ListProjectFiles500JSONResponse{
			Error: "unable to list project files",
		}, nil
	}

	projectFiles, err := s.projectFiles.List(
		ctx,
		session.UserID,
		googleuuid.UUID(request.ProjectId),
	)
	if err != nil {
		switch {
		case errors.Is(err, filestorage.ErrProjectIDRequired):
			return api.ListProjectFiles400JSONResponse{
				Error: "project ID is required",
			}, nil

		case errors.Is(err, filestorage.ErrProjectNotFound):
			return api.ListProjectFiles404JSONResponse{
				Error: "project not found",
			}, nil

		default:
			return api.ListProjectFiles500JSONResponse{
				Error: "unable to list project files",
			}, nil
		}
	}

	response := make(
		api.ListProjectFiles200JSONResponse,
		0,
		len(projectFiles),
	)

	for _, projectFile := range projectFiles {
		response = append(
			response,
			projectFileResponse(projectFile),
		)
	}

	return response, nil
}

func (s *Server) GetProjectFile(
	ctx context.Context,
	request api.GetProjectFileRequestObject,
) (api.GetProjectFileResponseObject, error) {
	if err := SessionResolutionError(ctx); err != nil {
		return api.GetProjectFile500JSONResponse{
			Error: "unable to authenticate request",
		}, nil
	}

	session, ok := SessionFromContext(ctx)
	if !ok {
		return api.GetProjectFile401JSONResponse{
			Error: "authentication required",
		}, nil
	}

	if s.projectFiles == nil {
		return api.GetProjectFile500JSONResponse{
			Error: "unable to get project file",
		}, nil
	}

	projectFile, err := s.projectFiles.Get(
		ctx,
		session.UserID,
		googleuuid.UUID(request.ProjectId),
		googleuuid.UUID(request.ProjectFileId),
	)
	if err != nil {
		switch {
		case errors.Is(err, filestorage.ErrProjectIDRequired):
			return api.GetProjectFile400JSONResponse{
				Error: "project ID is required",
			}, nil

		case errors.Is(err, filestorage.ErrProjectFileIDRequired):
			return api.GetProjectFile400JSONResponse{
				Error: "project file ID is required",
			}, nil

		case errors.Is(err, filestorage.ErrProjectNotFound):
			return api.GetProjectFile404JSONResponse{
				Error: "project not found",
			}, nil

		case errors.Is(err, filestorage.ErrProjectFileNotFound):
			return api.GetProjectFile404JSONResponse{
				Error: "project file not found",
			}, nil

		default:
			return api.GetProjectFile500JSONResponse{
				Error: "unable to get project file",
			}, nil
		}
	}

	return api.GetProjectFile200JSONResponse(
		projectFileResponse(projectFile),
	), nil
}

func (s *Server) DownloadProjectFileContent(
	ctx context.Context,
	request api.DownloadProjectFileContentRequestObject,
) (api.DownloadProjectFileContentResponseObject, error) {
	if err := SessionResolutionError(ctx); err != nil {
		return api.DownloadProjectFileContent500JSONResponse{
			Error: "unable to authenticate request",
		}, nil
	}

	session, ok := SessionFromContext(ctx)
	if !ok {
		return api.DownloadProjectFileContent401JSONResponse{
			Error: "authentication required",
		}, nil
	}

	if s.fileDownloads == nil {
		return api.DownloadProjectFileContent500JSONResponse{
			Error: "unable to download project file",
		}, nil
	}

	result, err := s.fileDownloads.Download(
		ctx,
		session.UserID,
		googleuuid.UUID(request.ProjectId),
		googleuuid.UUID(request.ProjectFileId),
	)
	if err != nil {
		switch {
		case errors.Is(err, filestorage.ErrProjectIDRequired):
			return api.DownloadProjectFileContent400JSONResponse{
				Error: "project ID is required",
			}, nil

		case errors.Is(err, filestorage.ErrProjectFileIDRequired):
			return api.DownloadProjectFileContent400JSONResponse{
				Error: "project file ID is required",
			}, nil

		case errors.Is(err, filestorage.ErrProjectNotFound):
			return api.DownloadProjectFileContent404JSONResponse{
				Error: "project not found",
			}, nil

		case errors.Is(err, filestorage.ErrProjectFileNotFound):
			return api.DownloadProjectFileContent404JSONResponse{
				Error: "project file not found",
			}, nil

		default:
			return api.DownloadProjectFileContent500JSONResponse{
				Error: "unable to download project file",
			}, nil
		}
	}

	if result.Content == nil {
		return api.DownloadProjectFileContent500JSONResponse{
			Error: "unable to download project file",
		}, nil
	}

	contentDisposition := mime.FormatMediaType(
		"attachment",
		map[string]string{
			"filename": result.ProjectFile.OriginalFilename,
		},
	)
	if contentDisposition == "" {
		_ = result.Content.Close()

		return api.DownloadProjectFileContent500JSONResponse{
			Error: "unable to download project file",
		}, nil
	}

	return api.DownloadProjectFileContent200ApplicationoctetStreamResponse{
		Body: result.Content,
		Headers: api.DownloadProjectFileContent200ResponseHeaders{
			ContentDisposition: &contentDisposition,
		},
	}, nil
}

var errInvalidMultipartUpload = errors.New(
	"invalid multipart upload",
)

type singleMultipartFileReader struct {
	part      *multipart.Part
	body      *multipart.Reader
	exhausted bool
}

func (r *singleMultipartFileReader) Read(
	buffer []byte,
) (int, error) {
	if r.exhausted {
		return 0, io.EOF
	}

	n, err := r.part.Read(buffer)
	if err == nil {
		return n, nil
	}

	if !errors.Is(err, io.EOF) {
		return n, errInvalidMultipartUpload
	}

	nextPart, nextErr := r.body.NextPart()
	switch {
	case errors.Is(nextErr, io.EOF):
		r.exhausted = true

		return n, io.EOF

	case nextErr != nil:
		return n, errInvalidMultipartUpload

	default:
		_ = nextPart.Close()

		return n, errInvalidMultipartUpload
	}
}

func (s *Server) UploadProjectFile(
	ctx context.Context,
	request api.UploadProjectFileRequestObject,
) (api.UploadProjectFileResponseObject, error) {
	if err := SessionResolutionError(ctx); err != nil {
		return api.UploadProjectFile500JSONResponse{
			Error: "unable to authenticate request",
		}, nil
	}

	session, ok := SessionFromContext(ctx)
	if !ok {
		return api.UploadProjectFile401JSONResponse{
			Error: "authentication required",
		}, nil
	}

	if s.fileUploads == nil {
		return api.UploadProjectFile500JSONResponse{
			Error: "unable to upload project file",
		}, nil
	}

	if request.Body == nil {
		return api.UploadProjectFile400JSONResponse{
			Error: "request body is required",
		}, nil
	}

	part, err := request.Body.NextPart()
	if errors.Is(err, io.EOF) {
		return api.UploadProjectFile400JSONResponse{
			Error: "file is required",
		}, nil
	}
	if err != nil {
		return api.UploadProjectFile400JSONResponse{
			Error: "invalid multipart upload",
		}, nil
	}
	defer part.Close()

	if part.FormName() != "file" {
		return api.UploadProjectFile400JSONResponse{
			Error: "file part is required",
		}, nil
	}

	originalFilename := strings.TrimSpace(part.FileName())
	if originalFilename == "" {
		return api.UploadProjectFile400JSONResponse{
			Error: "file name is required",
		}, nil
	}

	var mediaType *string

	if value := strings.TrimSpace(
		part.Header.Get("Content-Type"),
	); value != "" {
		mediaType = &value
	}

	projectFile, err := s.fileUploads.Upload(
		ctx,
		filestorage.UploadInput{
			OwnerUserID:      session.UserID,
			ProjectID:        googleuuid.UUID(request.ProjectId),
			OriginalFilename: originalFilename,
			MediaType:        mediaType,
			Source: &singleMultipartFileReader{
				part: part,
				body: request.Body,
			},
		},
	)
	if err != nil {
		switch {
		case errors.Is(err, filestorage.ErrProjectIDRequired):
			return api.UploadProjectFile400JSONResponse{
				Error: "project ID is required",
			}, nil

		case errors.Is(err, filestorage.ErrOriginalFilenameRequired):
			return api.UploadProjectFile400JSONResponse{
				Error: "file name is required",
			}, nil

		case errors.Is(err, filestorage.ErrUploadSourceRequired):
			return api.UploadProjectFile400JSONResponse{
				Error: "file is required",
			}, nil

		case errors.Is(err, errInvalidMultipartUpload):
			return api.UploadProjectFile400JSONResponse{
				Error: "invalid multipart upload",
			}, nil

		case errors.Is(err, filestorage.ErrProjectNotFound):
			return api.UploadProjectFile404JSONResponse{
				Error: "project not found",
			}, nil

		case errors.Is(
			err,
			filestorage.ErrContentObjectSizeConflict,
		):
			return api.UploadProjectFile409JSONResponse{
				Error: "stored content metadata conflict",
			}, nil

		default:
			return api.UploadProjectFile500JSONResponse{
				Error: "unable to upload project file",
			}, nil
		}
	}

	return api.UploadProjectFile201JSONResponse(
		projectFileResponse(projectFile),
	), nil
}

func projectFileResponse(
	projectFile dbgen.ProjectFile,
) api.ProjectFileResponse {
	return api.ProjectFileResponse{
		Id:               uuid.UUID(projectFile.ID),
		ProjectId:        uuid.UUID(projectFile.ProjectID),
		UploadedByUserId: uuid.UUID(projectFile.UploadedByUserID),
		ContentSha256:    projectFile.ContentSha256,
		OriginalFilename: projectFile.OriginalFilename,
		MediaType:        projectFile.MediaType,
		CreatedAt:        projectFile.CreatedAt,
	}
}

func apiUUIDFromPGUUID(value pgtype.UUID) *uuid.UUID {
	if !value.Valid {
		return nil
	}

	converted := uuid.UUID(value.Bytes)

	return &converted
}

func (s *Server) CreateProjectFileConversionJob(
	ctx context.Context,
	request api.CreateProjectFileConversionJobRequestObject,
) (api.CreateProjectFileConversionJobResponseObject, error) {
	if err := SessionResolutionError(ctx); err != nil {
		return api.CreateProjectFileConversionJob500JSONResponse{
			Error: "unable to authenticate request",
		}, nil
	}

	session, ok := SessionFromContext(ctx)
	if !ok {
		return api.CreateProjectFileConversionJob401JSONResponse{
			Error: "authentication required",
		}, nil
	}

	if s.conversionJobs == nil {
		return api.CreateProjectFileConversionJob500JSONResponse{
			Error: "unable to create conversion job",
		}, nil
	}

	job, err := s.conversionJobs.Create(
		ctx,
		conversionjobs.CreateInput{
			OwnerUserID:   session.UserID,
			ProjectID:     googleuuid.UUID(request.ProjectId),
			ProjectFileID: googleuuid.UUID(request.ProjectFileId),
		},
	)
	if err != nil {
		switch {
		case errors.Is(err, conversionjobs.ErrProjectIDRequired):
			return api.CreateProjectFileConversionJob400JSONResponse{
				Error: "project ID is required",
			}, nil

		case errors.Is(
			err,
			conversionjobs.ErrProjectFileIDRequired,
		):
			return api.CreateProjectFileConversionJob400JSONResponse{
				Error: "project file ID is required",
			}, nil

		case errors.Is(err, conversionjobs.ErrProjectNotFound):
			return api.CreateProjectFileConversionJob404JSONResponse{
				Error: "project not found",
			}, nil

		case errors.Is(
			err,
			conversionjobs.ErrProjectFileNotFound,
		):
			return api.CreateProjectFileConversionJob404JSONResponse{
				Error: "project file not found",
			}, nil

		case errors.Is(
			err,
			conversionjobs.ErrActiveConversionJobExists,
		):
			return api.CreateProjectFileConversionJob409JSONResponse{
				Error: "active conversion job already exists",
			}, nil

		default:
			return api.CreateProjectFileConversionJob500JSONResponse{
				Error: "unable to create conversion job",
			}, nil
		}
	}

	return api.CreateProjectFileConversionJob201JSONResponse(
		conversionJobResponse(job),
	), nil
}

func (s *Server) GetProjectConversionJob(
	ctx context.Context,
	request api.GetProjectConversionJobRequestObject,
) (api.GetProjectConversionJobResponseObject, error) {
	if err := SessionResolutionError(ctx); err != nil {
		return api.GetProjectConversionJob500JSONResponse{
			Error: "unable to authenticate request",
		}, nil
	}

	session, ok := SessionFromContext(ctx)
	if !ok {
		return api.GetProjectConversionJob401JSONResponse{
			Error: "authentication required",
		}, nil
	}

	if s.conversionJobs == nil {
		return api.GetProjectConversionJob500JSONResponse{
			Error: "unable to get conversion job",
		}, nil
	}

	job, err := s.conversionJobs.Get(
		ctx,
		session.UserID,
		googleuuid.UUID(request.ProjectId),
		googleuuid.UUID(request.ConversionJobId),
	)
	if err != nil {
		switch {
		case errors.Is(err, conversionjobs.ErrProjectIDRequired):
			return api.GetProjectConversionJob400JSONResponse{
				Error: "project ID is required",
			}, nil

		case errors.Is(
			err,
			conversionjobs.ErrConversionJobIDRequired,
		):
			return api.GetProjectConversionJob400JSONResponse{
				Error: "conversion job ID is required",
			}, nil

		case errors.Is(err, conversionjobs.ErrProjectNotFound):
			return api.GetProjectConversionJob404JSONResponse{
				Error: "project not found",
			}, nil

		case errors.Is(
			err,
			conversionjobs.ErrConversionJobNotFound,
		):
			return api.GetProjectConversionJob404JSONResponse{
				Error: "conversion job not found",
			}, nil

		default:
			return api.GetProjectConversionJob500JSONResponse{
				Error: "unable to get conversion job",
			}, nil
		}
	}

	return api.GetProjectConversionJob200JSONResponse(
		conversionJobResponse(job),
	), nil
}

func (s *Server) ListProjectFileConversionJobs(
	ctx context.Context,
	request api.ListProjectFileConversionJobsRequestObject,
) (api.ListProjectFileConversionJobsResponseObject, error) {
	if err := SessionResolutionError(ctx); err != nil {
		return api.ListProjectFileConversionJobs500JSONResponse{
			Error: "unable to authenticate request",
		}, nil
	}

	session, ok := SessionFromContext(ctx)
	if !ok {
		return api.ListProjectFileConversionJobs401JSONResponse{
			Error: "authentication required",
		}, nil
	}

	if s.conversionJobs == nil {
		return api.ListProjectFileConversionJobs500JSONResponse{
			Error: "unable to list conversion jobs",
		}, nil
	}

	jobs, err := s.conversionJobs.ListForProjectFile(
		ctx,
		session.UserID,
		googleuuid.UUID(request.ProjectId),
		googleuuid.UUID(request.ProjectFileId),
	)
	if err != nil {
		switch {
		case errors.Is(err, conversionjobs.ErrProjectIDRequired):
			return api.ListProjectFileConversionJobs400JSONResponse{
				Error: "project ID is required",
			}, nil

		case errors.Is(
			err,
			conversionjobs.ErrProjectFileIDRequired,
		):
			return api.ListProjectFileConversionJobs400JSONResponse{
				Error: "project file ID is required",
			}, nil

		case errors.Is(err, conversionjobs.ErrProjectNotFound):
			return api.ListProjectFileConversionJobs404JSONResponse{
				Error: "project not found",
			}, nil

		case errors.Is(
			err,
			conversionjobs.ErrProjectFileNotFound,
		):
			return api.ListProjectFileConversionJobs404JSONResponse{
				Error: "project file not found",
			}, nil

		default:
			return api.ListProjectFileConversionJobs500JSONResponse{
				Error: "unable to list conversion jobs",
			}, nil
		}
	}

	response := make(
		api.ListProjectFileConversionJobs200JSONResponse,
		0,
		len(jobs),
	)

	for _, job := range jobs {
		response = append(
			response,
			conversionJobResponse(job),
		)
	}

	return response, nil
}

func conversionJobResponse(
	job dbgen.ConversionJob,
) api.ConversionJobResponse {
	return api.ConversionJobResponse{
		Id:            uuid.UUID(job.ID),
		ProjectId:     uuid.UUID(job.ProjectID),
		ProjectFileId: uuid.UUID(job.ProjectFileID),
		Status: api.ConversionJobResponseStatus(
			job.Status,
		),
		AttemptCount: job.AttemptCount,
		LastError:    job.LastError,
		CreatedAt:    job.CreatedAt,
		StartedAt:    apiTimeFromPGTimestamptz(job.StartedAt),
		FinishedAt:   apiTimeFromPGTimestamptz(job.FinishedAt),
		UpdatedAt:    job.UpdatedAt,
	}
}

func apiTimeFromPGTimestamptz(
	value pgtype.Timestamptz,
) *time.Time {
	if !value.Valid {
		return nil
	}

	converted := value.Time

	return &converted
}

var _ ProjectService = (*projects.Service)(nil)
var _ VersioningService = (*versioning.Service)(nil)
var _ ProjectFileService = (*filestorage.Service)(nil)
var _ ProjectFileUploader = (*filestorage.UploadService)(nil)
var _ ProjectFileDownloader = (*filestorage.DownloadService)(nil)
var _ ConversionJobService = (*conversionjobs.Service)(nil)
var _ UserRegistrar = (*auth.RegistrationService)(nil)
var _ UserAuthenticator = (*auth.LoginService)(nil)
var _ SessionRevoker = (*auth.SessionService)(nil)
var _ CurrentUserReader = (*dbgen.Queries)(nil)
var _ api.StrictServerInterface = (*Server)(nil)
