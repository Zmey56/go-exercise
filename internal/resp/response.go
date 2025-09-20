package resp

import (
	"encoding/json"
	"net/http"
)

type Error struct {
	Message string `json:"message"`
}

func JSON(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}
