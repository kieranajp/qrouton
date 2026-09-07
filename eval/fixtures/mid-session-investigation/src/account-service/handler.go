package account

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
)

type Handler struct {
	service *Service
	config  Config
}

func NewHandler(config Config) *Handler {
	return &Handler{service: NewService(config), config: config}
}

func (h *Handler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	id := strings.TrimPrefix(request.URL.Path, "/accounts/")
	if id == "" {
		http.Error(writer, "missing account id", http.StatusBadRequest)
		return
	}

	ctx := request.Context()
	// A caller may shorten the retry budget but never lengthen it.
	if header := request.Header.Get("X-Retry-Budget-Ms"); header != "" {
		if budget, err := time.ParseDuration(header + "ms"); err == nil && budget < h.config.RetryDeadline {
			var cancel func()
			ctx, cancel = withDeadline(ctx, budget)
			defer cancel()
		}
	}

	account, err := h.service.Lookup(ctx, id)
	if err != nil {
		status := http.StatusBadGateway
		if errors.Is(err, ErrAttemptsExhausted) {
			status = http.StatusGatewayTimeout
		}
		http.Error(writer, err.Error(), status)
		return
	}
	writer.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(writer).Encode(account)
}
