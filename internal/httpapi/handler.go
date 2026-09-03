package httpapi

import (
	"net/http"

	api "github.com/AngelAvilesSil/3Default/internal/api"
)

func NewHandler(server api.StrictServerInterface) http.Handler {
	strictHandler := api.NewStrictHandler(server, nil)

	return api.Handler(strictHandler)
}
