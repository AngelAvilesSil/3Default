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

type ProjectCreator interface {
	CreateProject(
		ctx context.Context,
		arg dbgen.CreateProjectParams,
	) (dbgen.Project, error)
}

type Service struct {
	projects ProjectCreator
}

type CreateInput struct {
	OwnerUserID uuid.UUID
	Name        string
	Description *string
}

func NewService(projects ProjectCreator) *Service {
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

var _ ProjectCreator = (*dbgen.Queries)(nil)
