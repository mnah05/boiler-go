package handler

import "net/http"

func NotFoundHandler(w http.ResponseWriter, r *http.Request) {
	_ = NewErrorResponse(w, http.StatusNotFound, "not_found", "resource not found")
}
