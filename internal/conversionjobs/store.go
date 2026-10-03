package conversionjobs

import (
	"context"

	"github.com/AngelAvilesSil/3Default/internal/database/dbgen"
)

type Store interface {
	CreateConversionJob(
		ctx context.Context,
		arg dbgen.CreateConversionJobParams,
	) (dbgen.ConversionJob, error)
	GetConversionJobByIDAndProject(
		ctx context.Context,
		arg dbgen.GetConversionJobByIDAndProjectParams,
	) (dbgen.ConversionJob, error)
	ListConversionJobsByProjectFile(
		ctx context.Context,
		arg dbgen.ListConversionJobsByProjectFileParams,
	) ([]dbgen.ConversionJob, error)
}
