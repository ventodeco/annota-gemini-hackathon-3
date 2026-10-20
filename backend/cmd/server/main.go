package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/clerk/clerk-sdk-go/v2"
	"github.com/clerk/clerk-sdk-go/v2/jwks"
	"github.com/clerk/clerk-sdk-go/v2/user"
	_ "github.com/lib/pq"

	"github.com/gemini-hackathon/app/internal/ai"
	"github.com/gemini-hackathon/app/internal/auth"
	"github.com/gemini-hackathon/app/internal/config"
	"github.com/gemini-hackathon/app/internal/handlers"
	"github.com/gemini-hackathon/app/internal/knowledge"
	"github.com/gemini-hackathon/app/internal/logger"
	"github.com/gemini-hackathon/app/internal/middleware"
	"github.com/gemini-hackathon/app/internal/pdf"
	"github.com/gemini-hackathon/app/internal/storage"
	"github.com/gemini-hackathon/app/internal/usage"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	aiClient, err := buildAIClient(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize AI routes: %v", err)
	}

	db, err := sql.Open("postgres", cfg.DBConnectionString)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	if err := storage.RunMigrations(db, "migrations"); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	storageDB := storage.NewPostgresDB(db)

	fileStorage, err := storage.NewLocalFileStorage(cfg.UploadDir)
	if err != nil {
		log.Fatalf("Failed to create file storage: %v", err)
	}

	// Redis remains a deployment dependency (readiness, future caching) even
	// though Clerk sessions no longer store OAuth state in it.
	redisClient, err := storage.NewRedisClient(cfg.RedisAddr)
	if err != nil {
		log.Fatalf("Failed to connect to Redis at %s: %v", cfg.RedisAddr, err)
	}
	defer redisClient.Close()

	// Load knowledge service for vocabulary lookup
	var knowledgeSvc knowledge.Service
	if cfg.KnowledgeCSVPath != "" {
		var err error
		knowledgeSvc, err = knowledge.NewService(cfg.KnowledgeCSVPath)
		if err != nil {
			log.Printf("Warning: Failed to load knowledge CSV from %s: %v. Continuing without knowledge context.", cfg.KnowledgeCSVPath, err)
			knowledgeSvc = knowledge.NewEmptyService()
		} else {
			log.Printf("Loaded knowledge CSV from %s", cfg.KnowledgeCSVPath)
		}
	} else {
		knowledgeSvc = knowledge.NewEmptyService()
	}

	sessionVerifier, identityDirectory, err := buildClerkAuth(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize Clerk authentication: %v", err)
	}

	usageMeter := usage.NewMeter(storage.NewPostgresUsageStore(db), usage.LimitsFromConfig(cfg))

	userHandlers := handlers.NewUserHandlersWithStorage(storageDB, fileStorage, cfg, identityDirectory)
	scanHandlers := handlers.NewScanHandlers(storageDB, fileStorage, aiClient, cfg, usageMeter)
	aiHandlers := handlers.NewAIHandlers(storageDB, aiClient, knowledgeSvc, usageMeter)
	annotationHandlers := handlers.NewAnnotationHandlers(storageDB, cfg).WithAIClient(aiClient)
	analyticsHandlers := handlers.NewAnalyticsHandlers()
	entitlementHandlers := handlers.NewEntitlementHandlers(cfg, usageMeter)
	pdfExtractor := pdf.NewExtractor()
	documentHandlers := handlers.NewDocumentHandlers(storageDB, fileStorage, pdfExtractor, cfg)

	authMiddleware := middleware.NewAuthMiddleware(sessionVerifier, auth.NewIdentityResolver(storageDB, identityDirectory))
	aiRateLimiter := middleware.NewRateLimiter(
		cfg.AIRateLimit,
		time.Duration(cfg.AIRateLimitWindowSeconds)*time.Second,
	)

	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
	mux.HandleFunc("/readyz", handlers.NewReadinessHandler(2*time.Second,
		handlers.ReadinessCheck{Name: "postgres", Check: db.PingContext},
		handlers.ReadinessCheck{Name: "redis", Check: redisClient.Ping},
	))

	authMux := http.NewServeMux()
	authMux.HandleFunc("/v1/users/me/languages", userHandlers.GetLanguagesAPI)
	authMux.HandleFunc("/v1/users/me", userHandlers.UsersMeAPI)
	authMux.Handle("/v1/scans", aiRateLimiter.Handle(http.HandlerFunc(scanHandlers.ScansAPI)))
	authMux.HandleFunc("/v1/scans/", scanHandlers.GetScanAPI)
	authMux.Handle("/v1/ai/analyze", aiRateLimiter.Handle(http.HandlerFunc(aiHandlers.AnalyzeAPI)))
	authMux.Handle("/v1/ai/speech", aiRateLimiter.Handle(http.HandlerFunc(aiHandlers.SpeakAPI)))
	authMux.HandleFunc("/v1/annotations", annotationHandlers.AnnotationsAPI)
	authMux.HandleFunc("/v1/annotations/", annotationHandlers.AnnotationByIDAPI)
	authMux.HandleFunc("/v1/events", analyticsHandlers.EventsAPI)
	authMux.HandleFunc("/v1/entitlements/me", entitlementHandlers.MeAPI)
	authMux.HandleFunc("/v1/documents", documentHandlers.DocumentsAPI)
	authMux.HandleFunc("/v1/documents/", documentHandlers.DocumentByIDAPI)

	// Every API response is account data or an auth decision; never let browsers or proxies store it.
	mux.Handle("/v1/", middleware.NoStore(authMiddleware.Handle(authMux)))

	mux.Handle("/", handlers.NewFrontendHandler("web/dist"))

	corsMiddleware := middleware.NewCORSMiddleware(cfg)
	handler := middleware.LoggingMiddleware(corsMiddleware.Handle(mux))

	log.Printf("Server starting on :%s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, handler); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func buildClerkAuth(cfg *config.Config) (*auth.SessionVerifier, auth.IdentityDirectory, error) {
	secretKey := cfg.ClerkSecretKey
	clientConfig := &clerk.ClientConfig{BackendConfig: clerk.BackendConfig{Key: &secretKey}}

	keys := auth.NewJWKSKeySource(jwks.NewClient(clientConfig))
	if cfg.ClerkJWTKey != "" {
		staticKeys, err := auth.NewStaticKeySource(cfg.ClerkJWTKey)
		if err != nil {
			return nil, nil, err
		}
		keys = staticKeys
	}
	verifier, err := auth.NewSessionVerifier(auth.SessionVerifierConfig{
		Issuer:            cfg.ClerkIssuer,
		AuthorizedParties: cfg.ClerkAuthorizedParties,
		Keys:              keys,
		Leeway:            5 * time.Second,
	})
	if err != nil {
		return nil, nil, err
	}
	return verifier, auth.NewClerkDirectory(user.NewClient(clientConfig)), nil
}

func buildAIClient(cfg *config.Config) (ai.Client, error) {
	mode := strings.ToLower(strings.TrimSpace(cfg.AIProvider))
	if mode == ai.ProviderRouted {
		if cfg.AIRouting == nil {
			return nil, fmt.Errorf("routed AI configuration is missing")
		}
		return ai.NewRoutedClient(*cfg.AIRouting, ai.RoutedClientOptions{Observe: func(ctx context.Context, meta ai.CallMetadata) {
			logger.GetDefaultLogger().WithRequestID(middleware.GetRequestID(ctx)).WithField("ai", meta).Info("AI gateway attempt")
			usage.RecordCall(ctx, meta)
		}})
	}
	if mode != "" && mode != ai.ProviderGemini && mode != ai.ProviderMiniMax {
		return nil, fmt.Errorf("unsupported AI_PROVIDER")
	}
	return ai.NewClient(ai.ClientConfig{
		AIProvider:              cfg.AIProvider,
		GeminiAPIKey:            cfg.GeminiAPIKey,
		OpenRouterAPIKey:        cfg.OpenRouterAPIKey,
		OpenRouterBaseURL:       cfg.OpenRouterBaseURL,
		OpenRouterOCRModel:      cfg.OpenRouterOCRModel,
		MiniMaxAPIKey:           cfg.MiniMaxAPIKey,
		MiniMaxAnthropicBaseURL: cfg.MiniMaxAnthropicBaseURL,
		MiniMaxTextModel:        cfg.MiniMaxTextModel,
		MiniMaxTTSBaseURL:       cfg.MiniMaxTTSBaseURL,
		MiniMaxTTSModel:         cfg.MiniMaxTTSModel,
		MiniMaxTTSVoiceID:       cfg.MiniMaxTTSVoiceID,
	}), nil
}
