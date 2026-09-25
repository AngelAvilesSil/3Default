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
	ErrRevisionIDRequired = errors.New(
		"project revision ID is required",
	)
	ErrBranchIDRequired = errors.New(
		"project branch ID is required",
	)
	ErrBranchNameRequired = errors.New(
		"project branch name is required",
	)
	ErrHeadRevisionIDInvalid = errors.New(
		"branch head revision ID is invalid",
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
	ErrRevisionNotFound = errors.New(
		"project revision not found",
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

type CreateBranchInput struct {
	OwnerUserID    uuid.UUID
	ProjectID      uuid.UUID
	Name           string
	HeadRevisionID *uuid.UUID
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

func (s *Service) CreateBranch(
	ctx context.Context,
	input CreateBranchInput,
) (dbgen.ProjectBranch, error) {
	if input.OwnerUserID == uuid.Nil {
		return dbgen.ProjectBranch{}, ErrOwnerRequired
	}

	if input.ProjectID == uuid.Nil {
		return dbgen.ProjectBranch{}, ErrProjectIDRequired
	}

	name := strings.TrimSpace(input.Name)
	if name == "" {
		return dbgen.ProjectBranch{}, ErrBranchNameRequired
	}

	if input.HeadRevisionID != nil &&
		*input.HeadRevisionID == uuid.Nil {
		return dbgen.ProjectBranch{}, ErrHeadRevisionIDInvalid
	}

	_, err := s.projects.GetProjectByIDAndOwner(
		ctx,
		dbgen.GetProjectByIDAndOwnerParams{
			ProjectID:   input.ProjectID,
			OwnerUserID: input.OwnerUserID,
		},
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return dbgen.ProjectBranch{}, ErrProjectNotFound
	}
	if err != nil {
		return dbgen.ProjectBranch{}, fmt.Errorf(
			"get project for branch creation: %w",
			err,
		)
	}

	if input.HeadRevisionID != nil {
		_, err := s.revisions.GetProjectRevisionByIDAndProject(
			ctx,
			dbgen.GetProjectRevisionByIDAndProjectParams{
				RevisionID: *input.HeadRevisionID,
				ProjectID:  input.ProjectID,
			},
		)
		if errors.Is(err, pgx.ErrNoRows) {
			return dbgen.ProjectBranch{}, ErrRevisionNotFound
		}
		if err != nil {
			return dbgen.ProjectBranch{}, fmt.Errorf(
				"get branch head revision: %w",
				err,
			)
		}
	}

	branch, err := s.revisions.CreateBranch(
		ctx,
		CreateBranchParams{
			ProjectID:      input.ProjectID,
			Name:           name,
			HeadRevisionID: input.HeadRevisionID,
		},
	)
	if errors.Is(err, ErrBranchNameConflict) {
		return dbgen.ProjectBranch{}, ErrBranchNameConflict
	}
	if err != nil {
		return dbgen.ProjectBranch{}, fmt.Errorf(
			"create project branch: %w",
			err,
		)
	}

	return branch, nil
}

func (s *Service) ListBranches(
	ctx context.Context,
	ownerUserID uuid.UUID,
	projectID uuid.UUID,
) ([]dbgen.ProjectBranch, error) {
	if ownerUserID == uuid.Nil {
		return nil, ErrOwnerRequired
	}

	if projectID == uuid.Nil {
		return nil, ErrProjectIDRequired
	}

	_, err := s.projects.GetProjectByIDAndOwner(
		ctx,
		dbgen.GetProjectByIDAndOwnerParams{
			ProjectID:   projectID,
			OwnerUserID: ownerUserID,
		},
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrProjectNotFound
	}
	if err != nil {
		return nil, fmt.Errorf(
			"get project for branch listing: %w",
			err,
		)
	}

	branches, err := s.revisions.ListProjectBranchesByProject(
		ctx,
		projectID,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"list project branches: %w",
			err,
		)
	}

	if branches == nil {
		return []dbgen.ProjectBranch{}, nil
	}

	return branches, nil
}

func (s *Service) ListBranchHistory(
	ctx context.Context,
	ownerUserID uuid.UUID,
	projectID uuid.UUID,
	branchID uuid.UUID,
) ([]dbgen.ProjectRevision, error) {
	if ownerUserID == uuid.Nil {
		return nil, ErrOwnerRequired
	}

	if projectID == uuid.Nil {
		return nil, ErrProjectIDRequired
	}

	if branchID == uuid.Nil {
		return nil, ErrBranchIDRequired
	}

	_, err := s.projects.GetProjectByIDAndOwner(
		ctx,
		dbgen.GetProjectByIDAndOwnerParams{
			ProjectID:   projectID,
			OwnerUserID: ownerUserID,
		},
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrProjectNotFound
	}
	if err != nil {
		return nil, fmt.Errorf(
			"get project for branch history: %w",
			err,
		)
	}

	branch, err := s.revisions.GetProjectBranchByIDAndProject(
		ctx,
		dbgen.GetProjectBranchByIDAndProjectParams{
			BranchID:  branchID,
			ProjectID: projectID,
		},
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrBranchNotFound
	}
	if err != nil {
		return nil, fmt.Errorf(
			"get project branch for history: %w",
			err,
		)
	}

	if !branch.HeadRevisionID.Valid {
		return []dbgen.ProjectRevision{}, nil
	}

	headRevisionID := uuid.UUID(branch.HeadRevisionID.Bytes)

	revisions, err :=
		s.revisions.ListReachableProjectRevisionsFromRevision(
			ctx,
			dbgen.ListReachableProjectRevisionsFromRevisionParams{
				ProjectID:       projectID,
				StartRevisionID: headRevisionID,
			},
		)
	if err != nil {
		return nil, fmt.Errorf(
			"list project branch history: %w",
			err,
		)
	}

	if revisions == nil {
		return []dbgen.ProjectRevision{}, nil
	}

	return revisions, nil
}

func (s *Service) GetRevision(
	ctx context.Context,
	ownerUserID uuid.UUID,
	projectID uuid.UUID,
	revisionID uuid.UUID,
) (dbgen.ProjectRevision, error) {
	if ownerUserID == uuid.Nil {
		return dbgen.ProjectRevision{}, ErrOwnerRequired
	}

	if projectID == uuid.Nil {
		return dbgen.ProjectRevision{}, ErrProjectIDRequired
	}

	if revisionID == uuid.Nil {
		return dbgen.ProjectRevision{}, ErrRevisionIDRequired
	}

	_, err := s.projects.GetProjectByIDAndOwner(
		ctx,
		dbgen.GetProjectByIDAndOwnerParams{
			ProjectID:   projectID,
			OwnerUserID: ownerUserID,
		},
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return dbgen.ProjectRevision{}, ErrProjectNotFound
	}
	if err != nil {
		return dbgen.ProjectRevision{}, fmt.Errorf(
			"get project for revision lookup: %w",
			err,
		)
	}

	revision, err :=
		s.revisions.GetProjectRevisionByIDAndProject(
			ctx,
			dbgen.GetProjectRevisionByIDAndProjectParams{
				RevisionID: revisionID,
				ProjectID:  projectID,
			},
		)
	if errors.Is(err, pgx.ErrNoRows) {
		return dbgen.ProjectRevision{}, ErrRevisionNotFound
	}
	if err != nil {
		return dbgen.ProjectRevision{}, fmt.Errorf(
			"get project revision: %w",
			err,
		)
	}

	return revision, nil
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
