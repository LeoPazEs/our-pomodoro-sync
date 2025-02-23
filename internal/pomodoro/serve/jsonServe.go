package serve

import "net/http"

type JsonHandler interface {
	http.Handler
	JsonRequestHandler
	JsonResponseHandler
}

type JsonRequestHandler interface {
	jsonRequest(r *http.Request) http.Handler
}

type JsonResponseHandler interface {
	jsonResponse(w http.ResponseWriter)
	jsonErrorHandle(err JsonError, w http.ResponseWriter)
}

type JsonHandle struct {
	handler http.Handler
}

func (jh *JsonHandle) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	err := jh.jsonRequest(r)

	jh.jsonResponse(w)
	if err != nil {
		jh.jsonErrorHandle(err, w)
		return
	}
	jh.handler.ServeHTTP(w, r)
}

func (jh *JsonHandle) jsonRequest(r *http.Request) JsonError {
	if r.Header.Get("Content-Type") != "application/json" {
		return NewBadRequestError(nil, "Json endpoint.")
	}
	return nil
}

func (jh *JsonHandle) jsonResponse(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
}

func (jh *JsonHandle) jsonErrorHandle(err JsonError, w http.ResponseWriter) {
	w.WriteHeader(err.Code())
	w.Write([]byte(err.Error()))
}
