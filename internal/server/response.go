package server

import (
	"encoding/json"
	"net/http"
)

type Response struct {
	OK    bool        `json:"ok"`
	Data  interface{} `json:"data,omitempty"`
	Error string      `json:"error,omitempty"`
}

func writeJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	
	response := Response{
		OK:   statusCode < 400,
		Data: data,
	}
	
	json.NewEncoder(w).Encode(response)
}

func writeError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	
	response := Response{
		OK:    false,
		Error: message,
	}
	
	json.NewEncoder(w).Encode(response)
}