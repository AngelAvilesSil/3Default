package httpapi

import (
	"context"

	api "github.com/AngelAvilesSil/3Default/internal/api"
)

type Server struct{}

var _ api.StrictServerInterface = (*Server)(nil)

func NewServer() *Server {
	return &Server{}
}

func (s *Server) GetHealth(
	_ context.Context,
	_ api.GetHealthRequestObject,
) (api.GetHealthResponseObject, error) {
	return api.GetHealth200JSONResponse{
		Status: api.Ok,
	}, nil
}
