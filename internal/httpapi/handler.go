package httpapi

import (
	"encoding/json"
	"net/http"

	api "github.com/AngelAvilesSil/3Default/internal/api"
)

func NewHandler(
	server api.StrictServerInterface,
	sessionResolver SessionResolver,
) http.Handler {
	strictHandler := api.NewStrictHandlerWithOptions(
		server,
		nil,
		api.StrictHTTPServerOptions{
			RequestErrorHandlerFunc: func(
				w http.ResponseWriter,
				_ *http.Request,
				_ error,
			) {
				writeJSONError(
					w,
					http.StatusBadRequest,
					"invalid request",
				)
			},
		},
	)

	handler := api.Handler(strictHandler)

	handler = LogoutSessionTokenMiddleware(handler)

	handler = ProjectSessionContextMiddleware(
		sessionResolver,
	)(handler)

	crossOriginProtection := http.NewCrossOriginProtection()

	crossOriginProtection.SetDenyHandler(
		http.HandlerFunc(func(
			w http.ResponseWriter,
			_ *http.Request,
		) {
			writeJSONError(
				w,
				http.StatusForbidden,
				"cross-origin request denied",
			)
		}),
	)

	return crossOriginProtection.Handler(handler)
}

func writeJSONError(
	w http.ResponseWriter,
	status int,
	message string,
) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	_ = json.NewEncoder(w).Encode(
		api.ErrorResponse{
			Error: message,
		},
	)
}
