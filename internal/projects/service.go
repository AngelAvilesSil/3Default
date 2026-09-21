package projects

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/AngelAvilesSil/3Default/internal/database/dbgen"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var (
	ErrOwnerRequired     = errors.New("project owner is required")
	ErrProjectIDRequired = errors.New("project ID is required")
	ErrProjectNotFound   = errors.New("project not found")
	ErrNameRequired      = errors.New("project name is required")
	ErrNoMetadataChanges = errors.New("project metadata changes are required")
)

type ProjectStore interface {
	CreateProject(
		ctx context.Context,
		arg dbgen.CreateProjectParams,
	) (dbgen.Project, error)
	ListProjectsByOwner(
		ctx context.Context,
		ownerUserID uuid.UUID,
	) ([]dbgen.Project, error)
	GetProjectByIDAndOwner(
		ctx context.Context,
		arg dbgen.GetProjectByIDAndOwnerParams,
	) (dbgen.Project, error)
	UpdateProjectMetadataByIDAndOwner(
		ctx context.Context,
		arg dbgen.UpdateProjectMetadataByIDAndOwnerParams,
	) (dbgen.Project, error)
}

type Service struct {
	projects ProjectStore
}

type CreateInput struct {
	OwnerUserID uuid.UUID
	Name        string
	Description *string
}

type UpdateInput struct {
	OwnerUserID uuid.UUID
	ProjectID   uuid.UUID

	NameSet bool
	Name    *string

	DescriptionSet bool
	Description    *string
}

func NewService(projects ProjectStore) *Service {
	return &Service{
		projects: projects,
	}
}

func (s *Service) Create(
	ctx context.Context,
	input CreateInput,
) (dbgen.Project, error) {
	if input.OwnerUserID == uuid.Nil {
		return dbgen.Project{}, ErrOwnerRequired
	}

	name := strings.TrimSpace(input.Name)
	if name == "" {
		return dbgen.Project{}, ErrNameRequired
	}

	description := normalizeOptionalText(input.Description)

	project, err := s.projects.CreateProject(ctx, dbgen.CreateProjectParams{
		OwnerUserID: input.OwnerUserID,
		Name:        name,
		Description: description,
	})
	if err != nil {
		return dbgen.Project{}, fmt.Errorf("create project: %w", err)
	}

	return project, nil
}

func (s *Service) Get(
	ctx context.Context,
	ownerUserID uuid.UUID,
	projectID uuid.UUID,
) (dbgen.Project, error) {
	if ownerUserID == uuid.Nil {
		return dbgen.Project{}, ErrOwnerRequired
	}

	if projectID == uuid.Nil {
		return dbgen.Project{}, ErrProjectIDRequired
	}

	project, err := s.projects.GetProjectByIDAndOwner(
		ctx,
		dbgen.GetProjectByIDAndOwnerParams{
			ProjectID:   projectID,
			OwnerUserID: ownerUserID,
		},
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return dbgen.Project{}, ErrProjectNotFound
	}
	if err != nil {
		return dbgen.Project{}, fmt.Errorf(
			"get project: %w",
			err,
		)
	}

	return project, nil
}

func (s *Service) Update(
	ctx context.Context,
	input UpdateInput,
) (dbgen.Project, error) {
	if input.OwnerUserID == uuid.Nil {
		return dbgen.Project{}, ErrOwnerRequired
	}

	if input.ProjectID == uuid.Nil {
		return dbgen.Project{}, ErrProjectIDRequired
	}

	if !input.NameSet && !input.DescriptionSet {
		return dbgen.Project{}, ErrNoMetadataChanges
	}

	var name string
	if input.NameSet {
		if input.Name == nil {
			return dbgen.Project{}, ErrNameRequired
		}

		name = strings.TrimSpace(*input.Name)
		if name == "" {
			return dbgen.Project{}, ErrNameRequired
		}
	}

	var description *string
	if input.DescriptionSet {
		description = normalizeOptionalText(input.Description)
	}

	project, err := s.projects.UpdateProjectMetadataByIDAndOwner(
		ctx,
		dbgen.UpdateProjectMetadataByIDAndOwnerParams{
			NameSet:        input.NameSet,
			Name:           name,
			DescriptionSet: input.DescriptionSet,
			Description:    description,
			ProjectID:      input.ProjectID,
			OwnerUserID:    input.OwnerUserID,
		},
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return dbgen.Project{}, ErrProjectNotFound
	}
	if err != nil {
		return dbgen.Project{}, fmt.Errorf(
			"update project: %w",
			err,
		)
	}

	return project, nil
}

func (s *Service) List(
	ctx context.Context,
	ownerUserID uuid.UUID,
) ([]dbgen.Project, error) {
	if ownerUserID == uuid.Nil {
		return nil, ErrOwnerRequired
	}

	projects, err := s.projects.ListProjectsByOwner(
		ctx,
		ownerUserID,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"list projects: %w",
			err,
		)
	}

	if projects == nil {
		return []dbgen.Project{}, nil
	}

	return projects, nil
}

func normalizeOptionalText(value *string) *string {
	if value == nil {
		return nil
	}

	normalized := strings.TrimSpace(*value)
	if normalized == "" {
		return nil
	}

	return &normalized
}
