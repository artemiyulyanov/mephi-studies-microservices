package handlers

import (
	"database/sql"
	"errors"
	"net/http"
	"user-service/internal/database/repository"

	"github.com/gorilla/mux"

	"common/network"
)

type UserHandlers struct {
	repo *repository.UserRepository
}

func (handlers *UserHandlers) getUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	id, err := network.ParseID(r)

	if err != nil {
		network.RespondError(w, http.StatusBadRequest, "incorrect id!")
		return
	}

	user, err := handlers.repo.GetUserByID(ctx, id)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			network.RespondError(w, http.StatusNotFound, "user not found!")
		} else {
			network.RespondError(w, http.StatusBadRequest, err.Error())
		}
	}

	network.RespondJSON(w, http.StatusOK, user)
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
