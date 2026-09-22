package versioning

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
	ErrOwnerRequired = errors.New(
		"project owner is required",
	)
	ErrProjectIDRequired = errors.New(
		"project ID is required",
	)
	ErrBranchIDRequired = errors.New(
		"project branch ID is required",
	)
	ErrMessageRequired = errors.New(
		"revision message is required",
	)
	ErrExpectedHeadRevisionIDInvalid = errors.New(
		"expected head revision ID is invalid",
	)
	ErrMergeParentRevisionIDInvalid = errors.New(
		"merge parent revision ID is invalid",
	)
	ErrMergeRequiresHead = errors.New(
		"merge revision requires an expected branch head",
	)
	ErrRevisionParentsMustDiffer = errors.New(
		"revision parents must differ",
	)
	ErrProjectNotFound = errors.New(
		"project not found",
	)
)

type ProjectReader interface {
	GetProjectByIDAndOwner(
		ctx context.Context,
		arg dbgen.GetProjectByIDAndOwnerParams,
	) (dbgen.Project, error)
}

type Service struct {
	projects  ProjectReader
	revisions RevisionStore
}

type CreateRevisionInput struct {
	OwnerUserID uuid.UUID
	ProjectID   uuid.UUID
	BranchID    uuid.UUID
	Message     string

	ExpectedHeadRevisionID *uuid.UUID
	MergeParentRevisionID  *uuid.UUID
}

func NewService(
	projects ProjectReader,
	revisions RevisionStore,
) *Service {
	return &Service{
		projects:  projects,
		revisions: revisions,
	}
}

func (s *Service) CreateRevision(
	ctx context.Context,
	input CreateRevisionInput,
) (dbgen.ProjectRevision, error) {
	if input.OwnerUserID == uuid.Nil {
		return dbgen.ProjectRevision{}, ErrOwnerRequired
	}

	if input.ProjectID == uuid.Nil {
		return dbgen.ProjectRevision{}, ErrProjectIDRequired
	}

	if input.BranchID == uuid.Nil {
		return dbgen.ProjectRevision{}, ErrBranchIDRequired
	}

	message := strings.TrimSpace(input.Message)
	if message == "" {
		return dbgen.ProjectRevision{}, ErrMessageRequired
	}

	if input.ExpectedHeadRevisionID != nil &&
		*input.ExpectedHeadRevisionID == uuid.Nil {
		return dbgen.ProjectRevision{},
			ErrExpectedHeadRevisionIDInvalid
	}

	if input.MergeParentRevisionID != nil &&
		*input.MergeParentRevisionID == uuid.Nil {
		return dbgen.ProjectRevision{},
			ErrMergeParentRevisionIDInvalid
	}

	if input.MergeParentRevisionID != nil &&
		input.ExpectedHeadRevisionID == nil {
		return dbgen.ProjectRevision{},
			ErrMergeRequiresHead
	}

	if input.MergeParentRevisionID != nil &&
		input.ExpectedHeadRevisionID != nil &&
		*input.MergeParentRevisionID ==
			*input.ExpectedHeadRevisionID {
		return dbgen.ProjectRevision{},
			ErrRevisionParentsMustDiffer
	}

	_, err := s.projects.GetProjectByIDAndOwner(
		ctx,
		dbgen.GetProjectByIDAndOwnerParams{
			ProjectID:   input.ProjectID,
			OwnerUserID: input.OwnerUserID,
		},
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return dbgen.ProjectRevision{}, ErrProjectNotFound
	}
	if err != nil {
		return dbgen.ProjectRevision{}, fmt.Errorf(
			"get project for revision creation: %w",
			err,
		)
	}

	revision, err := s.revisions.CreateRevisionOnBranch(
		ctx,
		CreateRevisionOnBranchParams{
			ProjectID:              input.ProjectID,
			BranchID:               input.BranchID,
			AuthorUserID:           input.OwnerUserID,
			Message:                message,
			ExpectedHeadRevisionID: input.ExpectedHeadRevisionID,
			MergeParentRevisionID:  input.MergeParentRevisionID,
		},
	)
	if errors.Is(err, ErrBranchNotFound) {
		return dbgen.ProjectRevision{}, ErrBranchNotFound
	}
	if errors.Is(err, ErrBranchHeadConflict) {
		return dbgen.ProjectRevision{}, ErrBranchHeadConflict
	}
	if err != nil {
		return dbgen.ProjectRevision{}, fmt.Errorf(
			"create revision on branch: %w",
			err,
		)
	}

	return revision, nil
}
