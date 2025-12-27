package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/peregrine/router/internal/provider"
	"github.com/peregrine/router/internal/router"
)

type config struct {
	GRPCPort    int
	MetricsPort int

	// Ollama config
	OllamaEndpoint string
	OllamaModel    string
	OllamaTimeout  time.Duration

	// ngrok config
	NgrokEndpoint string
	NgrokAPIKey   string
	NgrokModel    string
	NgrokTimeout  time.Duration

	// Rules config
	RulesEnabled bool

	// Health config
	HealthInterval         time.Duration
	HealthFailureThreshold int
	HealthRecoveryTime     time.Duration
}

func main() {
	cfg := parseFlags()

	// Create providers
	providers := createProviders(cfg)

	// Create health config
	healthCfg := router.HealthConfig{
		Interval:         cfg.HealthInterval,
		FailureThreshold: cfg.HealthFailureThreshold,
		RecoveryTime:     cfg.HealthRecoveryTime,
	}

	// Create router
	r := router.NewRouter(providers, healthCfg)

	// Create gRPC server
	grpcServer := grpc.NewServer()

	// Register the LLM Router service
	svc := NewLLMRouterService(r)
	RegisterLLMRouterServer(grpcServer, svc)

	// Enable reflection for debugging
	reflection.Register(grpcServer)

	// Start metrics server
	go startMetricsServer(cfg.MetricsPort)

	// Start gRPC server
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.GRPCPort))
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to listen: %v\n", err)
		os.Exit(1)
	}

	// Start health checking
	ctx, cancel := context.WithCancel(context.Background())
	r.Start(ctx)

	// Handle shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		fmt.Println("\nShutting down...")
		cancel()
		r.Stop()
		grpcServer.GracefulStop()
	}()

	fmt.Printf("LLM Router starting on port %d (metrics: %d)\n", cfg.GRPCPort, cfg.MetricsPort)
	fmt.Printf("Providers: ")
	for _, p := range providers {
		fmt.Printf("%s (priority %d) ", p.Name(), p.Priority())
	}
	fmt.Println()

	if err := grpcServer.Serve(lis); err != nil {
		fmt.Fprintf(os.Stderr, "failed to serve: %v\n", err)
		os.Exit(1)
	}
}

func parseFlags() config {
	cfg := config{}

	flag.IntVar(&cfg.GRPCPort, "grpc-port", 50051, "gRPC server port")
	flag.IntVar(&cfg.MetricsPort, "metrics-port", 9090, "Prometheus metrics port")

	// Ollama
	flag.StringVar(&cfg.OllamaEndpoint, "ollama-endpoint",
		getEnv("OLLAMA_ENDPOINT", "http://localhost:11434"), "Ollama endpoint")
	flag.StringVar(&cfg.OllamaModel, "ollama-model",
		getEnv("OLLAMA_MODEL", "gemma3:270m"), "Ollama model")
	flag.DurationVar(&cfg.OllamaTimeout, "ollama-timeout", 60*time.Second, "Ollama timeout")

	// ngrok
	flag.StringVar(&cfg.NgrokEndpoint, "ngrok-endpoint",
		getEnv("NGROK_ENDPOINT", "https://ai-gateway.ngrok.io"), "ngrok AI Gateway endpoint")
	flag.StringVar(&cfg.NgrokAPIKey, "ngrok-api-key",
		getEnv("NGROK_API_KEY", ""), "ngrok API key")
	flag.StringVar(&cfg.NgrokModel, "ngrok-model",
		getEnv("NGROK_MODEL", "claude-3-sonnet"), "ngrok model")
	flag.DurationVar(&cfg.NgrokTimeout, "ngrok-timeout", 30*time.Second, "ngrok timeout")

	// Rules
	flag.BoolVar(&cfg.RulesEnabled, "rules-enabled", true, "Enable deterministic rules engine")

	// Health
	flag.DurationVar(&cfg.HealthInterval, "health-interval", 10*time.Second, "Health check interval")
	flag.IntVar(&cfg.HealthFailureThreshold, "health-failure-threshold", 3, "Failures before circuit opens")
	flag.DurationVar(&cfg.HealthRecoveryTime, "health-recovery-time", 30*time.Second, "Circuit breaker recovery time")

	flag.Parse()

	return cfg
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func createProviders(cfg config) []provider.Provider {
	providers := make([]provider.Provider, 0, 3)

	// Add ngrok provider if API key is set
	if cfg.NgrokAPIKey != "" {
		ngrokCfg := provider.NgrokConfig{
			Endpoint: cfg.NgrokEndpoint,
			APIKey:   cfg.NgrokAPIKey,
			Model:    cfg.NgrokModel,
			Timeout:  cfg.NgrokTimeout,
		}
		providers = append(providers, provider.NewNgrokProvider(ngrokCfg))
	}

	// Add Ollama provider
	ollamaCfg := provider.OllamaConfig{
		Endpoint:           cfg.OllamaEndpoint,
		Model:              cfg.OllamaModel,
		Timeout:            cfg.OllamaTimeout,
		ConfidenceThreshold: 0.7,
	}
	providers = append(providers, provider.NewOllamaProvider(ollamaCfg))

	// Add rules provider
	if cfg.RulesEnabled {
		rulesCfg := provider.DefaultRulesConfig()
		providers = append(providers, provider.NewRulesProvider(rulesCfg))
	}

	return providers
}

func startMetricsServer(port int) {
	http.Handle("/metrics", promhttp.Handler())
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	addr := fmt.Sprintf(":%d", port)
	fmt.Printf("Metrics server listening on %s\n", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		fmt.Fprintf(os.Stderr, "metrics server error: %v\n", err)
	}
}

// LLMRouterService implements the gRPC service
type LLMRouterService struct {
	UnimplementedLLMRouterServer
	router *router.Router
}

// NewLLMRouterService creates a new service instance
func NewLLMRouterService(r *router.Router) *LLMRouterService {
	return &LLMRouterService{router: r}
}

// Note: The actual gRPC methods are implemented in service.go
// after proto generation. This is a placeholder until then.
