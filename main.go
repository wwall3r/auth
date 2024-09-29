package main

import (
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/joho/godotenv"

	"github.com/gorilla/pat"
	"github.com/gorilla/sessions"
	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
	"github.com/markbates/goth/providers/amazon"
	"github.com/markbates/goth/providers/apple"
	"github.com/markbates/goth/providers/discord"
	"github.com/markbates/goth/providers/facebook"
	"github.com/markbates/goth/providers/google"
	"github.com/markbates/goth/providers/twitch"
	"github.com/markbates/goth/providers/twitterv2"
	"github.com/wwall3r/signedcookie"
)

func main() {
	if err := initEverything(); err != nil {
		log.Fatal(err)
	}

	host := os.Getenv("AUTH_HOST")
	port := os.Getenv("PORT")

	// see the full list of available providers at
	// https://github.com/markbates/goth/blob/master/examples/main.go
	goth.UseProviders(
		amazon.New(os.Getenv("AMAZON_KEY"), os.Getenv("AMAZON_SECRET"), host+"/auth/amazon/callback"),
		apple.New(os.Getenv("APPLE_KEY"), os.Getenv("APPLE_SECRET"), host+"/auth/apple/callback", nil, apple.ScopeName, apple.ScopeEmail),
		discord.New(os.Getenv("DISCORD_KEY"), os.Getenv("DISCORD_SECRET"), host+"/auth/discord/callback", discord.ScopeIdentify, discord.ScopeEmail),
		facebook.New(os.Getenv("FACEBOOK_KEY"), os.Getenv("FACEBOOK_SECRET"), host+"/auth/facebook/callback"),
		google.New(os.Getenv("GOOGLE_KEY"), os.Getenv("GOOGLE_SECRET"), host+"/auth/google/callback"),
		twitch.New(os.Getenv("TWITCH_KEY"), os.Getenv("TWITCH_SECRET"), host+"/auth/twitch/callback"),

		// Use twitterv2 instead of twitter if you only have access to the Essential API Level
		// the twitter provider uses a v1.1 API that is not available to the Essential Level
		twitterv2.New(os.Getenv("TWITTER_KEY"), os.Getenv("TWITTER_SECRET"), host+"/auth/twitterv2/callback"),
		// If you'd like to use authenticate instead of authorize in TwitterV2 provider, use this instead.
		// twitterv2.NewAuthenticate(os.Getenv("TWITTER_KEY"), os.Getenv("TWITTER_SECRET"), "http://localhost:3000/auth/twitterv2/callback"),
	)

	router := pat.New()

	router.Get("/auth/{provider}/callback", Make(func(res http.ResponseWriter, req *http.Request) error {
		user, err := gothic.CompleteUserAuth(res, req)
		if err != nil {
			return err
		}

		onLoggedIn(res, req, user)
		return nil
	}))

	router.Get("/logout/{provider}", Make(func(res http.ResponseWriter, req *http.Request) error {
		outputCookie.RemoveValues(res, "user")

		// remove refresh token from long session
		longValues, _ := outputCookie.GetValues(req, res, "long", longModifier)
		longValues["refreshToken"] = ""
		outputCookie.SetValues(res, "long", longValues, longModifier)

		gothic.Logout(res, req)
		redirectTo(res, req)
		return nil
	}))

	router.Get("/auth/{provider}", Make(func(res http.ResponseWriter, req *http.Request) error {
		// try to get the user without re-authenticating
		if gothUser, err := gothic.CompleteUserAuth(res, req); err == nil {
			onLoggedIn(res, req, gothUser)
		} else {
			saveRedirectTo(res, req)
			gothic.BeginAuthHandler(res, req)
		}
		return nil
	}))

	slog.Info("Starting auth server", "port", port)
	log.Fatal(http.ListenAndServe(port, router))
}

var store sessions.Store
var outputCookie *signedcookie.SignedCookie

func initEverything() error {
	if err := godotenv.Load(); err != nil {
		return err
	}

	gothic.Store = createCookieStore()
	store = gothic.Store

	outputCookie = createSignedCookie()
	return nil
}

func createCookieStore() sessions.Store {
	authUrl, err := url.Parse(os.Getenv("AUTH_HOST"))
	if err != nil {
		slog.Error("AUTH_HOST is not a valid URL")
		return nil
	}

	cookieStore := sessions.NewCookieStore([]byte(os.Getenv("SESSION_SECRET")))
	cookieStore.Options.HttpOnly = true
	cookieStore.Options.Secure = authUrl.Scheme == "https"
	cookieStore.Options.SameSite = http.SameSiteLaxMode
	cookieStore.Options.Domain = getApexDomain(authUrl.Hostname())
	return cookieStore
}

func createSignedCookie() *signedcookie.SignedCookie {
	authUrl, err := url.Parse(os.Getenv("AUTH_HOST"))
	if err != nil {
		slog.Error("AUTH_HOST is not a valid URL")
		return nil
	}

	sc := signedcookie.New(os.Getenv("SESSION_SECRET"))

	sc.CookieOptions.HttpOnly = true
	sc.CookieOptions.Secure = authUrl.Scheme == "https"
	sc.CookieOptions.SameSite = http.SameSiteLaxMode
	sc.CookieOptions.Domain = getApexDomain(authUrl.Hostname())

	return &sc
}

func Make(handler func(http.ResponseWriter, *http.Request) error) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if err := handler(writer, request); err != nil {
			slog.Error("internal server error", "err", err, "path", request.URL.Path)
			http.Error(writer, "internal server error", http.StatusInternalServerError)
		}
	}
}

func sessionModifier(cookie *http.Cookie) {
	cookie.MaxAge = 60 * 60 * 24 * 30 // TODO: make this configurable
}

func longModifier(cookie *http.Cookie) {
	cookie.MaxAge = 60 * 60 * 24 * 365 // TODO: make this configurable
}

func redirectToModifier(cookie *http.Cookie) {
	cookie.MaxAge = 60 * 60
}

// TODO: figure out what to do with the refresh token. goth mentions support here:
// https://github.com/markbates/goth?tab=readme-ov-file#security-notes, however, gothic
// defers a call to Logout() during CompleteUserAuth(). gothic does not use the refresh
// token, so we need to make a lower-level call to goth to use it.
func onLoggedIn(res http.ResponseWriter, req *http.Request, gothUser goth.User) {
	// TODO: the serialization of the stores here needs to be compatible with other
	// languages. Find a replacement for encoding/gob (or see if that is easy to read
	// elsewhere) and see how easy that is to wire into the store.
	// (If we pair down the info, can we just use strings directly?)
	longValues, err := outputCookie.GetValues(req, res, "long", longModifier)
	if err != nil {
		slog.Error("could not get long cookie", "err", err)
	}

	// this helps applications know the last provider which was used so they
	// can surface that on the login page
	longValues["provider"] = gothUser.Provider
	longValues["refreshToken"] = gothUser.RefreshToken
	outputCookie.SetValues(res, "long", longValues, longModifier)
	slog.Info("GOT HERE", "longValues", longValues)

	// save the user to a session cookie store
	sessionValues, err := outputCookie.GetValues(req, res, "user", sessionModifier)
	if err != nil {
		slog.Error("could not get session cookie", "err", err)
	}
	sessionValues["userId"] = gothUser.UserID
	sessionValues["email"] = gothUser.Email
	outputCookie.SetValues(res, "user", sessionValues, sessionModifier)

	redirectTo(res, req)
}

func saveRedirectTo(res http.ResponseWriter, req *http.Request) {
	redirectTo := req.URL.Query().Get("redirect")

	if !validateRedirectTo(redirectTo) {
		http.Error(res, "Invalid redirectTo", http.StatusBadRequest)
		return
	}

	redirectValues := make(signedcookie.CookieValues)
	slog.Info("saving redirectTo cookie", "redirectTo", redirectTo)
	redirectValues["redirectTo"] = redirectTo
	outputCookie.SetValues(res, "redirectTo", redirectValues, redirectToModifier)
}

func validateRedirectTo(redirectTo string) bool {
	redirectUrl, err := url.Parse(redirectTo)
	if err != nil {
		slog.Error("redirectTo is not a valid URL")
		return false
	}

	appUrl, err := url.Parse(os.Getenv("AUTH_HOST"))
	if err != nil {
		slog.Error("appUrl is not a valid URL")
		return false
	}

	if redirectUrl.Scheme != appUrl.Scheme {
		slog.Error("redirectTo scheme does not match auth scheme", redirectUrl.Scheme, appUrl.Scheme)
		return false
	}

	redirectApexDomain := getApexDomain(redirectUrl.Hostname())
	appApexDomain := getApexDomain(appUrl.Hostname())

	if redirectApexDomain != appApexDomain {
		slog.Error("redirectTo apex domain does not match auth apex domain")
		return false
	}

	return true
}

func getApexDomain(str string) string {
	domainParts := strings.Split(str, ".")
	apexParts := domainParts[max(len(domainParts)-2, 0):]
	return strings.Join(apexParts, ".")
}

func redirectTo(res http.ResponseWriter, req *http.Request) {
	// get redirect URL from params or cookie
	redirectTo := req.URL.Query().Get("redirect")

	if redirectTo == "" {
		redirectToValues, _ := outputCookie.GetValues(req, res, "redirectTo", redirectToModifier)
		fmt.Printf("%#v\n", redirectToValues)
		redirectTo = getString(redirectToValues["redirectTo"], "")
	}

	slog.Info("redirecting", "redirectTo", redirectTo)

	if validateRedirectTo(redirectTo) {
		// remove redirectTo cookie
		outputCookie.RemoveValues(res, "redirectTo")
		http.Redirect(res, req, redirectTo, http.StatusTemporaryRedirect)
	} else {
		http.Error(res, "Invalid redirectTo", http.StatusBadRequest)
	}
}

func getString(value interface{}, defaultValue string) string {
	fmt.Printf("%#v\n", value)

	if value, ok := value.(string); ok {
		return value
	}

	return defaultValue
}
