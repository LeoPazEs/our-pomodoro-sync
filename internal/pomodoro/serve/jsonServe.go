package serve

import (
	"errors"
	"net/http"
)

func jsonRequest(r *http.Request) error {
	if r.Header.Get("Content-Type") != "application/json" {
		return errors.New("This is a json endpoint.")
	}
	return nil
}

func JsonHandleFunc(
	handler func(w http.ResponseWriter, r *http.Request) JsonError,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := handler(w, r); err != nil {
			jsonErrorHandle(err, w)
		}
	})
}

func jsonErrorHandle(err JsonError, w http.ResponseWriter) {
	w.WriteHeader(err.Code())
	w.Write([]byte(err.Error()))
}
