package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	ory "github.com/ory/client-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ory/terraform-provider-ory/internal/testutil"
)

// oauth2GatewayUnauthorizedBody is the oryapis.com gateway's object-"error" 401
// body, which the OAuth2 SDK cannot decode into ErrorOAuth2 (error is a string),
// so it returns an opaque unmarshal error the wrapper must enrich with the body.
const oauth2GatewayUnauthorizedBody = `{"error":{"code":401,"status":"Unauthorized",` +
	`"request":"rid-9999","message":"Access credentials are invalid"}}`

func oauth2Unauthorized401Server() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(oauth2GatewayUnauthorizedBody))
	}))
}

func oauth2TestClient(t *testing.T, url string) *OryClient {
	t.Helper()
	c, err := NewOryClient(OryClientConfig{
		ProjectAPIKey: testutil.TestProjectAPIKey,
		ProjectSlug:   testutil.TestProjectSlug,
		ProjectAPIURL: url + "/%s",
	})
	require.NoError(t, err)
	return c
}

// TestOAuth2Clients_SurfaceErrorBodyOnAllVerbs verifies that every OAuth2 client
// wrapper (Get/Update/Delete — Create is covered by
// TestCreateOAuth2Client_SurfacesErrorBody) routes through wrapAPIError and
// surfaces the HTTP status, request ID, and raw response body rather than the
// opaque SDK decode error.
func TestOAuth2Clients_SurfaceErrorBodyOnAllVerbs(t *testing.T) {
	assertSurfaced := func(t *testing.T, op string, err error) {
		t.Helper()
		require.Error(t, err)
		msg := err.Error()
		assert.Contains(t, msg, op, "should name the operation")
		assert.Contains(t, msg, "401", "should surface the HTTP status")
		assert.Contains(t, msg, "Access credentials are invalid", "should surface the raw response body")
		assert.Contains(t, msg, "rid-9999", "should surface the request ID for Ory support")
	}

	t.Run("Get", func(t *testing.T) {
		srv := oauth2Unauthorized401Server()
		defer srv.Close()
		_, err := oauth2TestClient(t, srv.URL).GetOAuth2Client(context.Background(), "cid")
		assertSurfaced(t, "reading OAuth2 client", err)
	})

	t.Run("Update", func(t *testing.T) {
		srv := oauth2Unauthorized401Server()
		defer srv.Close()
		_, err := oauth2TestClient(t, srv.URL).UpdateOAuth2Client(context.Background(), "cid", ory.OAuth2Client{})
		assertSurfaced(t, "updating OAuth2 client", err)
	})

	t.Run("Delete", func(t *testing.T) {
		srv := oauth2Unauthorized401Server()
		defer srv.Close()
		err := oauth2TestClient(t, srv.URL).DeleteOAuth2Client(context.Background(), "cid")
		assertSurfaced(t, "deleting OAuth2 client", err)
	})
}
