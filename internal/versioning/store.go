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
	ErrBranchHeadConflict = errors.New(
		"project branch head changed",
	)
)

type CreateRevisionOnBranchParams struct {
	ProjectID    uuid.UUID
	BranchID     uuid.UUID
	AuthorUserID uuid.UUID
	Message      string

	ExpectedHeadRevisionID *uuid.UUID
	MergeParentRevisionID  *uuid.UUID
}

type RevisionStore interface {
	CreateRevisionOnBranch(
		ctx context.Context,
		arg CreateRevisionOnBranchParams,
	) (dbgen.ProjectRevision, error)
	GetProjectRevisionByIDAndProject(
		ctx context.Context,
		arg dbgen.GetProjectRevisionByIDAndProjectParams,
	) (dbgen.ProjectRevision, error)
	ListProjectBranchesByProject(
		ctx context.Context,
		projectID uuid.UUID,
	) ([]dbgen.ProjectBranch, error)
}
