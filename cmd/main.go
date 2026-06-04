package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"fluid/probes/core"
	"fluid/probes/core/controlplane"
	"fluid/probes/core/enroll"
	"fluid/probes/debian/internal/config"
	"fluid/probes/debian/internal/probe"
)

const (
	envEnrollmentToken      = "FLUID_ENROLLMENT_TOKEN"
	envControlplaneHTTPBase = "FLUID_CONTROLPLANE_HTTP_BASE"
)

func main() {
	configPath := flag.String("config", "/etc/fluid-probe/config.yml", "Path to probe configuration file")
	credentialsPath := flag.String("credentials", "/etc/fluid-probe/credentials.yaml", "Durable credentials (organization_uuid, token, base_url)")
	enrollmentEnvPath := flag.String("enrollment-env", "/etc/fluid-probe/enrollment.env", "systemd EnvironmentFile removed after enrollment when FLUID_ENROLL_PURGE_ENROLLMENT_SOURCES is true")
	flag.Parse()

	cfg, err := config.ParseConfigFile(*configPath)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	if err := config.MergeCredentialsFromFile(cfg, *credentialsPath); err != nil {
		log.Fatalf("failed to merge credentials: %v", err)
	}

	if v := strings.TrimSpace(os.Getenv(envControlplaneHTTPBase)); v != "" {
		cfg.Controlplane.BaseURL = v
	}

	enrollSecret := strings.TrimSpace(os.Getenv(envEnrollmentToken))
	httpBase := strings.TrimSpace(os.Getenv(envControlplaneHTTPBase))

	if strings.TrimSpace(cfg.Auth.Token) == "" && enrollSecret != "" {
		if httpBase == "" {
			log.Fatalf("enrollment requires %s to be set (e.g. via %s)", envControlplaneHTTPBase, *enrollmentEnvPath)
		}
		extra, err := enroll.ExtraArgsFromEnv()
		if err != nil {
			log.Fatalf("enrollment: %v", err)
		}
		hostname, _ := os.Hostname()
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		enrollName := strings.TrimSpace(cfg.Probe.Name)
		res, err := enroll.Exchange(ctx, enroll.Params{
			BaseURL:         httpBase,
			EnrollmentToken: enrollSecret,
			Hostname:        hostname,
			Name:            enrollName,
			Principal:       enroll.PrincipalProbe,
			AgentType:       "debian",
			ExtraArgs:       extra,
		})
		if err != nil {
			log.Fatalf("enrollment failed: %v", err)
		}
		cfg.Auth.OrganizationUUID = res.OrganizationUUID
		cfg.Auth.Token = res.ConnectionToken
		if strings.TrimSpace(cfg.Controlplane.BaseURL) == "" {
			cfg.Controlplane.BaseURL = httpBase
		}
		if err := config.WriteCredentialsFile(*credentialsPath, cfg); err != nil {
			log.Fatalf("failed to persist credentials: %v", err)
		}
		if enroll.PurgeEnrollmentSources() {
			p := strings.TrimSpace(*enrollmentEnvPath)
			if p != "" {
				if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
					log.Fatalf("failed to remove enrollment env file %s: %v", p, err)
				}
				log.Printf("enrollment succeeded, credentials written and enrollment env removed")
			}
		} else {
			log.Printf("enrollment succeeded, credentials written (FLUID_ENROLL_PURGE_ENROLLMENT_SOURCES=false)")
		}
	}

	rt, err := config.NewRuntime(cfg)
	if err != nil {
		log.Fatalf("invalid configuration: %v", err)
	}

	cpClient, err := controlplane.NewHTTPClient(
		cfg.Controlplane.BaseURL,
		cfg.Auth.OrganizationUUID,
		cfg.Auth.Token,
		cfg.Probe.Name,
		cfg.Probe.Version,
	)
	if err != nil {
		log.Fatalf("control plane client: %v", err)
	}
	runtimeSync := controlplane.NewRuntimeSync(cpClient)

	p, err := probe.New(rt, cpClient, runtimeSync)
	if err != nil {
		log.Fatalf("failed to initialize probe: %v", err)
	}

	schemaPath := ""
	if _, err := core.LoadSchemaFromConfigDir(*configPath); err == nil {
		schemaPath = filepath.Join(filepath.Dir(*configPath), "schema.yml")
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	go func() {
		<-ctx.Done()
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer stopCancel()
		if err := p.Stop(stopCtx); err != nil {
			log.Printf("probe stop warning: %v", err)
		}
	}()

	if err := p.Start(ctx, schemaPath); err != nil {
		log.Printf("probe exited with error: %v", err)
		os.Exit(1)
	}
}
