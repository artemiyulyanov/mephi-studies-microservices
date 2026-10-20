package handlers

import (
	"net/http"
	"user-service/internal/database/repository"

	"github.com/gorilla/mux"
)

type UserHandlers struct {
	repo *repository.UserRepository
}

func (handlers *UserHandlers) getUser(w http.ResponseWriter, r *http.Request) {
	// ctx := r.Context()

	// users, err :=
}

func (handlers *UserHandlers) registerRoutes(router *mux.Router) {
	router.HandleFunc("/{id}", handlers.getUser).Methods("GET")
}

func RegisterUserHandlers(router *mux.Router, repo *repository.UserRepository) *UserHandlers {
	handlers := UserHandlers{
		repo: repo,
	}

	handlers.registerRoutes(router)

	return &handlers
}
