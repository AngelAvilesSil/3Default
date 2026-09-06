package httpapi

import (
	"encoding/json"
	"net/http"

	api "github.com/AngelAvilesSil/3Default/internal/api"
)

func NewHandler(server api.StrictServerInterface) http.Handler {
	strictHandler := api.NewStrictHandlerWithOptions(
		server,
		nil,
		api.StrictHTTPServerOptions{
			RequestErrorHandlerFunc: func(
				w http.ResponseWriter,
				_ *http.Request,
				_ error,
			) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)

				_ = json.NewEncoder(w).Encode(
					api.ErrorResponse{
						Error: "invalid request",
					},
				)
			},
		},
	)

	return api.Handler(strictHandler)
}
