package handler

import (
	"context"
	"github.com/gorilla/sessions"
	"log/slog"

	// this should import your User type
	"myapp/types"

	"net/http"
	"net/url"
	"os"
)

func WithUser(next http.Handler) http.Handler {
	fn := func(writer http.ResponseWriter, request *http.Request) {
		session := getSession(request)
		longSession := getLongSession(request)

		userId := getDefaultValue(session.Values["userId"], "")
		email := getDefaultValue(session.Values["email"], "")
		provider := getDefaultValue(longSession.Values["provider"], "")

		user := types.User{
			UserID:   userId,
			Email:    email,
			Provider: provider,
		}

		if user.UserID != "" {
			slog.Info("user found in session", "user", user)
			ctx := context.WithValue(request.Context(), types.UserContextKey, user)
			request = request.WithContext(ctx)
		}

		next.ServeHTTP(writer, request)
	}

	return http.HandlerFunc(fn)
}

func WithRequireAuth(next http.Handler) http.Handler {
	fn := func(writer http.ResponseWriter, request *http.Request) {
		user := getUser(request)

		if user.UserID == "" {
			queryParams := url.Values{}
			queryParams.Set("redirect", request.URL.Path)

			// redirect to login page
			http.Redirect(writer, request, "/login?"+queryParams.Encode(), http.StatusSeeOther)
			return
		}

		next.ServeHTTP(writer, request)
	}

	return http.HandlerFunc(fn)
}

// The rest of these are more utilities than middleware, but I've included them
// here for completeness.

var store sessions.Store

func createStore(request *http.Request) {
	store = sessions.NewCookieStore([]byte(os.Getenv("SESSION_SECRET")))
}

func getSession(request *http.Request) *sessions.Session {
	if store == nil {
		createStore(request)
	}

	session, _ := store.Get(request, "user")
	return session
}

func getLongSession(request *http.Request) *sessions.Session {
	if store == nil {
		createStore(request)
	}

	session, _ := store.Get(request, "long")
	return session
}

func getUser(request *http.Request) types.User {
	user, ok := request.Context().Value(types.UserContextKey).(types.User)

	if !ok {
		return types.User{}
	}

	return user
}

func getDefaultValue(value interface{}, defaultValue string) string {
	if value, ok := value.(string); ok {
		return value
	}

	return defaultValue
}
