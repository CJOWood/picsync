package googlephotos

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type GooglephotosAuth struct {
	Access oauth2.Token `json:"access,omitempty"`
}

func newOauth2Config(consumerKey string, consumerSecret string, redirectUrl string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     consumerKey,
		ClientSecret: consumerSecret,
		RedirectURL:  redirectUrl,
		Scopes: []string{
			// Picker API scope to let the user select media items
			"https://www.googleapis.com/auth/photospicker.mediaitems.readonly",
			// Library API scope to fetch the selected media items
			"https://www.googleapis.com/auth/photoslibrary.readonly.appcreateddata",
		},
		Endpoint: google.Endpoint,
	}
}

func waitForCode(cc *CodeCatcher) string {
	for {
		select {
		case code := <-cc.Codes:
			// On success, give the server a chance to send some HTML
			// back to the user.  The login flow works even if this doesn't
			// happen, but it prevents the user from staring at an error
			// page in their browser.
			time.Sleep(2 * time.Second)
			cc.Server.Close()
			return code
		case err := <-cc.Errors:
			fmt.Printf("Error while waiting for code: %s\n", err.Error())
		}
	}
}

// Login does the OAuth2 login flow to google photos, resulting in Access tokens
func Login(consumerKey string, consumerSecret string) (*oauth2.Token, error) {

	// Google only allows OAuth2 via callback (even to localhost), it no longer
	// allows "OOB" OAuth2 flows (to mitigate phishing).  So we must start up a
	// web server to catch the code from the user's browser.
	codeCatcher, err := newCodeCatcher()
	if err != nil {
		return nil, err
	}

	config := newOauth2Config(consumerKey, consumerSecret, codeCatcher.CatcherURL)

	authURL := config.AuthCodeURL(
		codeCatcher.State,
		oauth2.AccessTypeOffline,
		oauth2.ApprovalForce,
	)
	fmt.Printf("Follow this link to authorize:\n%s\n\n", authURL)
	code := waitForCode(codeCatcher)
	fmt.Printf("Successfully got one-time code from OAuth2 login, exchanging for tokens\n")
	token, err := config.Exchange(context.TODO(), code, oauth2.AccessTypeOffline)
	if err != nil {
		return nil, err
	}
	return token, nil
}

// ServeLogin runs an HTTP server that guides the user through the OAuth2 login
// flow. It listens on listenAddr and prints the resulting credentials to stdout
// once authorization completes.
func ServeLogin(consumerKey, consumerSecret, listenAddr string) error {
	state := uuid.NewString()

	mux := http.NewServeMux()

	var cfg *oauth2.Config
	srv := &http.Server{Addr: listenAddr, Handler: mux}

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		redirect := fmt.Sprintf("http://%s/callback", r.Host)
		cfg = newOauth2Config(consumerKey, consumerSecret, redirect)
		url := cfg.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.ApprovalForce)
		fmt.Fprintf(w, "<a href=%q>Login with Google</a>", url)
	})

	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		if r.Form.Get("state") != state {
			http.Error(w, "state mismatch", 400)
			return
		}
		code := r.Form.Get("code")
		token, err := cfg.Exchange(r.Context(), code)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		fmt.Fprintf(w, "Login complete. You can close this window.")
		fmt.Printf("googlephotos.access.token_type: %s\n", token.TokenType)
		fmt.Printf("googlephotos.access.access_token: %s\n", token.AccessToken)
		fmt.Printf("googlephotos.access.refresh_token: %s\n", token.RefreshToken)
		fmt.Printf("googlephotos.access.expiry: %s\n", token.Expiry.Format(time.RFC3339))
		go func() {
			// give browser time to read message
			time.Sleep(2 * time.Second)
			srv.Shutdown(context.Background())
		}()
	})

	fmt.Printf("Open http://%s in your browser to authenticate\n", listenAddr)
	return srv.ListenAndServe()
}
