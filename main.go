package main

import (
	"log"
	"os"
	"time"

	"github.com/faroedev/faroe"
)

type actionLogger struct{}

func (*actionLogger) LogActionError(timestamp time.Time, message string, actionInvocationId string, action string) {
	log.Printf("[%s] action=%s (id %s) - %s", timestamp.Format("2006-01-02 15:04:05.000"), action, actionInvocationId, message)
}

func main() {
	// Load environment variables from .env file
	envMap, err := loadEnv(".env")
	if err != nil {
		log.Printf("Warning: Could not load .env file: %v", err)
		envMap = make(map[string]string)
	}

	// Get configuration from environment variables
	dbPath := getEnvWithMap(envMap, "FAROE_DB_PATH")
	port := getEnvWithMap(envMap, "FAROE_PORT")
	userActionInvocationURL := getEnvWithMap(envMap, "FAROE_USER_ACTION_INVOCATION_URL")
	smtpSenderName := getEnvWithMap(envMap, "FAROE_SMTP_SENDER_NAME")
	smtpSenderEmail := getEnvWithMap(envMap, "FAROE_SMTP_SENDER_EMAIL")
	smtpServerHost := getEnvWithMap(envMap, "FAROE_SMTP_SERVER_HOST")
	smtpServerPort := getEnvWithMap(envMap, "FAROE_SMTP_SERVER_PORT")
	smtpDomain := getEnvWithMap(envMap, "FAROE_SMTP_DOMAIN")

	storage := newStorage(dbPath)
	defer storage.Close()
	userActionInvocationClient := newUserActionInvocationClient(userActionInvocationURL)
	userServerClient := faroe.NewUserServerClient(userActionInvocationClient)
	userPasswordHashAlgorithm := newArgon2id(3, 1024*64, 1)
	temporaryPasswordHashAlgorithm := newArgon2id(3, 1024*16, 1)
	emailSender, err := newSmtpEmailSender(&smtpConfig{
		senderName:  smtpSenderName,
		senderEmail: smtpSenderEmail,
		serverHost:  smtpServerHost,
		serverPort:  smtpServerPort,
		ipVersion:   IPv4,
		domain:      smtpDomain,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer emailSender.Close()

	stopKeepAlive := emailSender.StartKeepAliveRoutine(time.Minute * 5)
	defer func() {
		log.Println("Stopping keep-alive routine...")
		stopKeepAlive <- true
		close(stopKeepAlive)
	}()

	faroeServer := faroe.NewServer(
		storage,
		userServerClient,
		&actionLogger{},
		[]faroe.PasswordHashAlgorithmInterface{userPasswordHashAlgorithm},
		temporaryPasswordHashAlgorithm,
		1,
		faroe.RealClock,
		faroe.AllowAllEmailAddresses,
		emailSender,
		faroe.SessionConfigStruct{
			InactivityTimeout:     30 * 24 * time.Hour,
			ActivityCheckInterval: time.Minute,
		},
	)

	server := &serverStruct{server: faroeServer}

	server.listen(port)
}

func getEnvWithMap(envMap map[string]string, key string) string {
	if value, exists := envMap[key]; exists && value != "" {
		return value
	}
	if value := os.Getenv(key); value != "" {
		return value
	}
	log.Fatalf("Required environment variable %s not found", key)
	return ""
}
