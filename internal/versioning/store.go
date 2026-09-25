package versioning

import (
	"context"
	"errors"

	"github.com/AngelAvilesSil/3Default/internal/database/dbgen"
	"github.com/google/uuid"
)

var (
	ErrBranchNotFound = errors.New(
		"project branch not found",
	)
	ErrBranchNameConflict = errors.New(
		"project branch name already exists",
	)
	ErrBranchHeadConflict = errors.New(
		"project branch head changed",
	)
)

type CreateBranchParams struct {
	ProjectID      uuid.UUID
	Name           string
	HeadRevisionID *uuid.UUID
}

type CreateRevisionOnBranchParams struct {
	ProjectID    uuid.UUID
	BranchID     uuid.UUID
	AuthorUserID uuid.UUID
	Message      string

	ExpectedHeadRevisionID *uuid.UUID
	MergeParentRevisionID  *uuid.UUID
}

type RevisionStore interface {
	CreateBranch(
		ctx context.Context,
		arg CreateBranchParams,
	) (dbgen.ProjectBranch, error)
	CreateRevisionOnBranch(
		ctx context.Context,
		arg CreateRevisionOnBranchParams,
	) (dbgen.ProjectRevision, error)
	GetProjectBranchByIDAndProject(
		ctx context.Context,
		arg dbgen.GetProjectBranchByIDAndProjectParams,
	) (dbgen.ProjectBranch, error)
	GetProjectRevisionByIDAndProject(
		ctx context.Context,
		arg dbgen.GetProjectRevisionByIDAndProjectParams,
	) (dbgen.ProjectRevision, error)
	ListProjectBranchesByProject(
		ctx context.Context,
		projectID uuid.UUID,
	) ([]dbgen.ProjectBranch, error)
	ListReachableProjectRevisionsFromRevision(
		ctx context.Context,
		arg dbgen.ListReachableProjectRevisionsFromRevisionParams,
	) ([]dbgen.ProjectRevision, error)
}
