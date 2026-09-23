package httpapi

import (
	"context"
	"errors"
	"time"
	"uuid"

	api "github.com/AngelAvilesSil/3Default/internal/api"
	"github.com/AngelAvilesSil/3Default/internal/auth"
	"github.com/AngelAvilesSil/3Default/internal/database/dbgen"
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
	CreateRevision(
		ctx context.Context,
		input versioning.CreateRevisionInput,
	) (dbgen.ProjectRevision, error)
	ListBranches(
		ctx context.Context,
		ownerUserID googleuuid.UUID,
		projectID googleuuid.UUID,
	) ([]dbgen.ProjectBranch, error)
	GetRevision(
		ctx context.Context,
		ownerUserID googleuuid.UUID,
		projectID googleuuid.UUID,
		revisionID googleuuid.UUID,
	) (dbgen.ProjectRevision, error)
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

	if !request.Body.ExpectedHeadRevisionId.IsSpecified() {
		return api.CreateProjectRevision400JSONResponse{
			Error: "expected head revision ID is required",
		}, nil
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

		case errors.Is(err, versioning.ErrProjectNotFound):
			return api.CreateProjectRevision404JSONResponse{
				Error: "project not found",
			}, nil

		case errors.Is(err, versioning.ErrBranchNotFound):
			return api.CreateProjectRevision404JSONResponse{
				Error: "branch not found",
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

func apiUUIDFromPGUUID(value pgtype.UUID) *uuid.UUID {
	if !value.Valid {
		return nil
	}

	converted := uuid.UUID(value.Bytes)

	return &converted
}

var _ ProjectService = (*projects.Service)(nil)
var _ VersioningService = (*versioning.Service)(nil)
var _ UserRegistrar = (*auth.RegistrationService)(nil)
var _ UserAuthenticator = (*auth.LoginService)(nil)
var _ SessionRevoker = (*auth.SessionService)(nil)
var _ CurrentUserReader = (*dbgen.Queries)(nil)
var _ api.StrictServerInterface = (*Server)(nil)
