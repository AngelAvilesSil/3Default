package httpapi

import (
	"context"
	"errors"
	"time"
	"uuid"

	api "github.com/AngelAvilesSil/3Default/internal/api"
	"github.com/AngelAvilesSil/3Default/internal/database/dbgen"
	"github.com/AngelAvilesSil/3Default/internal/projects"
)

type DatabasePinger interface {
	Ping(context.Context) error
}

type ProjectCreator interface {
	Create(
		ctx context.Context,
		input projects.CreateInput,
	) (dbgen.Project, error)
}

type Server struct {
	database DatabasePinger
	projects ProjectCreator
}

func NewServer(
	database DatabasePinger,
	projects ProjectCreator,
) *Server {
	return &Server{
		database: database,
		projects: projects,
	}
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

var _ ProjectCreator = (*projects.Service)(nil)
var _ api.StrictServerInterface = (*Server)(nil)
