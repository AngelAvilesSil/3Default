package httpapi

import (
	"context"
	"time"

	api "github.com/AngelAvilesSil/3Default/internal/api"
)

type DatabasePinger interface {
	Ping(context.Context) error
}

type Server struct {
	database DatabasePinger
}

var _ api.StrictServerInterface = (*Server)(nil)

func NewServer(database DatabasePinger) *Server {
	return &Server{
		database: database,
	}
}

func (s *Server) GetHealth(
	_ context.Context,
	_ api.GetHealthRequestObject,
) (api.GetHealthResponseObject, error) {
	return api.GetHealth200JSONResponse{
		Status: api.Ok,
	}, nil
}

func (s *Server) GetReady(
	ctx context.Context,
	_ api.GetReadyRequestObject,
) (api.GetReadyResponseObject, error) {
	pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	if err := s.database.Ping(pingCtx); err != nil {
		return api.GetReady503JSONResponse{
			Status: api.Unavailable,
		}, nil
	}

	return api.GetReady200JSONResponse{
		Status: api.Ready,
	}, nil
}
