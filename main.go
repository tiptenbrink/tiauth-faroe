package main

import (
	"flag"
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
	// Parse command line flags
	insecure := flag.Bool("insecure", false, "Disable TLS encryption for SMTP (dangerous)")
	envFile := flag.String("env-file", ".env", "Path to environment file")
	interactive := flag.Bool("interactive", false, "Run in interactive mode with stdin commands")
	noSmtpInit := flag.Bool("no-smtp-init", false, "Do not initialize SMTP connection on startup")
	noKeepAlive := flag.Bool("no-keep-alive", false, "Do not run keep-alive routine")
	enableReset := flag.Bool("enable-reset", false, "Enable request to /reset to clear storage")
	flag.Parse()

	// Load environment variables from specified env file
	envMap, err := loadEnv(*envFile)
	if err != nil {
		log.Printf("Warning: Could not load env file %s: %v", *envFile, err)
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
	var smtpSecurity SMTPSecurity
	if *insecure {
		smtpSecurity = SMTPInsecureDangerous
	} else {
		smtpSecurity = SMTPSecure
	}

	emailConfig := &smtpConfig{
		senderName:       smtpSenderName,
		senderEmail:      smtpSenderEmail,
		serverHost:       smtpServerHost,
		serverPort:       smtpServerPort,
		ipVersion:        IPv4,
		domain:           smtpDomain,
		security:         smtpSecurity,
		disableKeepAlive: *noKeepAlive,
	}
	var emailSender *smtpActionsEmailSender

	emailSender = &smtpActionsEmailSender{
		config: emailConfig,
	}
	if *noSmtpInit {
		// Don't initialize
	} else {
		emailSender.m.Lock()
		err := emailSender.Start(time.Minute * 5)
		if err != nil {
			log.Fatalf("failed to start server: %v", err)
		}
		emailSender.m.Unlock()
	}
	defer emailSender.Close()

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

	server := &serverStruct{server: faroeServer, storage: storage, enableReset: *enableReset}
	server.listen(port)
	// TODO: probably should create separate connection for db
	shell := NewInteractiveShell(storage)
	if *interactive {
		shell.listen()
	}

	for {
		select {
		case serverErr := <-server.errChan:
			log.Fatal(serverErr)
		case shellErr := <-shell.errChan:
			log.Fatal(shellErr)
		case mailErr := <-emailSender.errChan:
			log.Fatal(mailErr)
		}
	}
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
