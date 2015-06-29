package handlers

import (
	"errors"
	"log"
	"net/http"

	userStore "user_storage"

	"dash"
)

func getUserFromSession(u userStore.Storage, req *http.Request) (dash.User, error) {
	var sessionID = ""
	for _, cookie := range req.Cookies() {
		if cookie.Name == "laravel_session" {
			sessionID = cookie.Value
		}
	}
	if sessionID == "" {
		log.Printf("no laravel_session cookie found")
		return dash.User{}, errors.New("no session")
	}
	log.Printf("session: %q", sessionID)
	return u.FindByRememberToken(sessionID)
}
