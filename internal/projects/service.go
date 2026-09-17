package projects

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/AngelAvilesSil/3Default/internal/database/dbgen"
	"github.com/google/uuid"
)

var (
	ErrOwnerRequired = errors.New("project owner is required")
	ErrNameRequired  = errors.New("project name is required")
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
}

type Service struct {
	projects ProjectStore
}

type CreateInput struct {
	OwnerUserID uuid.UUID
	Name        string
	Description *string
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

var _ ProjectStore = (*dbgen.Queries)(nil)
