// Command autosecrets-core is the AutoSecrets control plane service.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"autosecrets.dev/core/internal/app"
	"autosecrets.dev/core/internal/config"
	"autosecrets.dev/core/internal/crypto"
	"autosecrets.dev/core/internal/database"
	"autosecrets.dev/core/internal/identity"
)

var version = "dev"

func main() {
	cfg, err := config.FromEnv()
	if err != nil {
		log.Fatalf("configuration error: %v", err)
	}
	cfg.Version = config.BuildVersion(version)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	keysDir := config.KeysDir()
	st, err := database.Connect(ctx, config.DatabaseDSN())
	if err != nil {
		log.Fatalf("database error: %v", err)
	}
	defer st.Close()
	if err := st.ValidateSingleAdministrator(ctx); err != nil {
		log.Fatalf("identity data error: %v", err)
	}

	mk, err := crypto.LoadOrCreateMasterKey(keysDir)
	if err != nil {
		log.Fatalf("master key error: %v", err)
	}
	ca, err := crypto.LoadOrCreateCA(keysDir)
	if err != nil {
		log.Fatalf("agent CA error: %v", err)
	}
	signer, err := crypto.LoadOrCreateSigner(keysDir)
	if err != nil {
		log.Fatalf("signing key error: %v", err)
	}
	var oidcClient *identity.OIDCClient
	oidcUnavailable := ""
	if err := cfg.OIDCConfigurationError(); err != nil {
		oidcUnavailable = err.Error()
	} else {
		oidcClient, err = identity.DiscoverOIDC(ctx, identity.OIDCConfig{
			PublicURL: cfg.PublicURL, IssuerURL: cfg.OIDCIssuerURL, ClientID: cfg.OIDCClientID,
			ClientSecret: cfg.OIDCClientSecret, Scopes: cfg.OIDCScopes,
		})
		if err != nil {
			oidcUnavailable = err.Error()
			log.Printf("OIDC unavailable; local login remains enabled: %v", err)
		}
	}
	var oauthClient *identity.OAuthClient
	oauthUnavailable := ""
	if err := cfg.OAuthConfigurationError(); err != nil {
		oauthUnavailable = err.Error()
	} else {
		oauthClient, err = identity.NewOAuthClient(identity.OAuthConfig{
			PublicURL: cfg.PublicURL, AuthorizationURL: cfg.OAuthAuthorizationURL, TokenURL: cfg.OAuthTokenURL,
			UserinfoURL: cfg.OAuthUserinfoURL, ClientID: cfg.OAuthClientID, ClientSecret: cfg.OAuthClientSecret,
			Scopes: cfg.OAuthScopes,
		})
		if err != nil {
			oauthUnavailable = err.Error()
			log.Printf("OAuth unavailable; local login remains enabled: %v", err)
		}
	}

	application := app.New(st, mk, ca, signer, cfg.ManagementBase, cfg.AgentBase, app.Options{
		Version:          cfg.Version,
		PublicURL:        cfg.PublicURL,
		PublicAgentURL:   config.PublicAgentURL(),
		ArtifactDir:      config.ArtifactDir(),
		InstallCurlOpts:  config.InstallCurlOpts(),
		TrustedProxy:     cfg.TrustedProxyCIDRs,
		CertHeader:       cfg.ProxyCertHeader,
		CORSOrigins:      cfg.CORSOrigins,
		OIDCClient:       oidcClient,
		OIDCUnavailable:  oidcUnavailable,
		OAuthClient:      oauthClient,
		OAuthUnavailable: oauthUnavailable,
	})
	if _, err := application.EmitBootstrapCode(ctx); err != nil {
		log.Fatalf("bootstrap code error: %v", err)
	}

	handler := application.Handler()
	httpServer := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		log.Printf("autosecrets-core listening on %s (management %s, agent %s)", cfg.ListenAddr, cfg.ManagementBase, cfg.AgentBase)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server error: %v", err)
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown error: %v", err)
	}
}
