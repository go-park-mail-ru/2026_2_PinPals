package httpx

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
)

const MaxJSONBodySize int64 = 2 << 20 // 2 MiB

func DecodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	contentType := r.Header.Get("Content-Type")
	if contentType != "" {
		mediaType, _, err := mime.ParseMediaType(contentType)
		if err != nil || mediaType != "application/json" {
			WriteError(w, http.StatusUnsupportedMediaType, "тип содержимого должен быть application/json")
			return errors.New("invalid content type")
		}
	}
	r.Body = http.MaxBytesReader(w, r.Body, MaxJSONBodySize)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			WriteError(w, http.StatusRequestEntityTooLarge, "тело запроса слишком большое")
		} else {
			WriteError(w, http.StatusBadRequest, "некорректный JSON")
		}
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		WriteError(w, http.StatusBadRequest, "тело запроса должно содержать один JSON-объект")
		return errors.New("extra json value")
	}
	return nil
}
