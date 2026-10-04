package httpx

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
)

func DecodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	contentType := r.Header.Get("Content-Type")
	if contentType != "" && !strings.HasPrefix(contentType, "application/json") {
		WriteError(w, http.StatusUnsupportedMediaType, "content type must be application/json")
		return errors.New("invalid content type")
	}

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(dst); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid json body")
		return err
	}

	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		WriteError(w, http.StatusBadRequest, "request body must contain one json object")
		return errors.New("extra json value")
	}

	return nil
}
