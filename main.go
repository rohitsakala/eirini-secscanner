package main

import (
	"fmt"
	"log"
	"os"
	"strconv"

	eirinix "code.cloudfoundry.org/eirinix"
	"go.uber.org/zap"
)

const operatorFingerprint = "eirini-secscanner"

var appVersion string = ""

func main() {
	z, err := zap.NewProduction()
	if err != nil {
		log.Fatal(fmt.Errorf("could not create logger: %w", err))
	}
	defer z.Sync()
	zaplog := z.Sugar()

	zaplog.Infow("Starting eirini-secscanner", "version", appVersion)
	eiriniNsEnvVar := os.Getenv("EIRINI_NAMESPACE")
	if eiriniNsEnvVar == "" {
		zaplog.Fatal("the EIRINI_NAMESPACE environment variable must be set")
	}

	webhookNsEnvVar := os.Getenv("EXTENSION_NAMESPACE")
	if webhookNsEnvVar == "" {
		zaplog.Fatal("the EXTENSION_NAMESPACE environment variable must be set")
	}

	portEnvVar := os.Getenv("PORT")
	if portEnvVar == "" {
		zaplog.Fatal("the PORT environment variable must be set")
	}
	port, err := strconv.Atoi(portEnvVar)
	if err != nil {
		zaplog.Fatalw("could not convert port to integer", "error", err, "port", portEnvVar)
	}

	serviceNameEnvVar := os.Getenv("SERVICE_NAME")
	if serviceNameEnvVar == "" {
		zaplog.Fatal("the SERVICE_NAME environment variable must be set")
	}
	severity := os.Getenv("SEVERITY")
	if severity == "" {
		severity = "CRITICAL"
	}

	filter := true

	ext := eirinix.NewManager(eirinix.ManagerOptions{
		Namespace:           eiriniNsEnvVar,
		Host:                "0.0.0.0",
		Port:                int32(port),
		Logger:              zaplog,
		FilterEiriniApps:    &filter,
		OperatorFingerprint: operatorFingerprint,
		ServiceName:         serviceNameEnvVar,
		WebhookNamespace:    webhookNsEnvVar,
	})

	ext.AddExtension(&Extension{Memory: os.Getenv("MEMORY"), Severity: severity})

	if err := ext.Start(); err != nil {
		zaplog.Fatalw("error starting eirinix manager", "error", err)
	}

	zaplog.Info("eirini-secscanner started")
}
