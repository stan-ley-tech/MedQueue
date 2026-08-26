package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/stan-ley-tech/medqueue/internal/apperr"
	"github.com/stan-ley-tech/medqueue/internal/domain"
	"github.com/stan-ley-tech/medqueue/internal/httpserver"
	"github.com/stan-ley-tech/medqueue/internal/service"
	"github.com/stan-ley-tech/medqueue/internal/validation"
)

// decodeAndValidate decodes the request body into dst and runs struct
// validation. It caps the body size to guard against a client streaming
// an oversized payload at a JSON endpoint.
func decodeAndValidate(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MiB
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return apperr.Validation("request body is not valid JSON", map[string]string{"body": err.Error()})
	}
	return validation.Struct(dst)
}

func actorFromRequest(r *http.Request) (service.Actor, error) {
	claims, ok := httpserver.ClaimsFromContext(r.Context())
	if !ok {
		return service.Actor{}, apperr.Unauthorized("")
	}
	return service.Actor{UserID: claims.UserID, Role: claims.Role, DoctorID: claims.DoctorID}, nil
}

func parsePage(r *http.Request) domain.Page {
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))
	page := domain.Page{Limit: limit, Offset: offset}
	page.NormalizeDefaults()
	return page
}
