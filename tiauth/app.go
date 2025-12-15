package tiauth

import (
	"fmt"
	"log"
	"os"
	"strings"
	"text/template"
	"time"

	"github.com/faroedev/faroe"
)

// ActionLogger implements faroe's action logging interface
type ActionLogger struct{}

func (*ActionLogger) LogActionError(timestamp time.Time, message string, actionInvocationId string, action string) {
	log.Printf("[%s] action=%s (id %s) - %s", timestamp.Format("2006-01-02 15:04:05.000"), action, actionInvocationId, message)
}

// App represents the running tiauth-faroe application
type App struct {
	config           Config
	storage          *storageStruct
	emailSender      *smtpActionsEmailSender
	tokenBroadcaster *TokenBroadcaster
	httpServer       *httpServer
	shell            *interactiveShell
}

// Run starts the tiauth-faroe server with the given configuration.
// This is a blocking call that runs until an error occurs.
func Run(cfg Config) error {
	app := &App{config: cfg}

	// Validate required config
	if cfg.UserActionInvocationURL == "" {
		return fmt.Errorf("config error: UserActionInvocationURL is required (set FAROE_USER_ACTION_INVOCATION_URL in env file or environment)")
	}

	// Determine if SMTP should be used
	smtpEnabled := !cfg.DisableSMTP
	if smtpEnabled {
		// Validate SMTP config only if SMTP is enabled
		if cfg.SMTPSenderEmail == "" {
			return fmt.Errorf("config error: SMTPSenderEmail is required when SMTP is enabled (set FAROE_SMTP_SENDER_EMAIL or use --no-smtp)")
		}
		if cfg.SMTPServerHost == "" {
			return fmt.Errorf("config error: SMTPServerHost is required when SMTP is enabled (set FAROE_SMTP_SERVER_HOST or use --no-smtp)")
		}
		if cfg.SMTPServerPort == "" {
			return fmt.Errorf("config error: SMTPServerPort is required when SMTP is enabled (set FAROE_SMTP_SERVER_PORT or use --no-smtp)")
		}
		if cfg.SMTPDomain == "" {
			return fmt.Errorf("config error: SMTPDomain is required when SMTP is enabled (set FAROE_SMTP_DOMAIN or use --no-smtp)")
		}
	} else {
		log.Println("SMTP disabled - emails will not be sent, only tokens will be broadcast")
	}

	// Initialize storage
	app.storage = newStorage(cfg.DBPath)
	defer app.storage.Close()

	// Load private route access key if configured
	var privateRouteAccessKey string
	if cfg.PrivateRouteKeyFile != "" {
		keyBytes, err := os.ReadFile(cfg.PrivateRouteKeyFile)
		if err != nil {
			return fmt.Errorf("failed to read private route key file: %v", err)
		}
		privateRouteAccessKey = strings.TrimSpace(string(keyBytes))
		log.Printf("Loaded private route access key from %s", cfg.PrivateRouteKeyFile)
	}

	// Initialize user action client
	userActionInvocationClient := newUserActionInvocationClient(cfg.UserActionInvocationURL, privateRouteAccessKey)
	userServerClient := faroe.NewUserServerClient(userActionInvocationClient)

	// Initialize password hash algorithms
	userPasswordHashAlgorithm := newArgon2id(3, 1024*64, 1)
	temporaryPasswordHashAlgorithm := newArgon2id(3, 1024*16, 1)

	// Initialize token broadcaster
	app.tokenBroadcaster = NewTokenBroadcaster(cfg.TokenSocketPath)
	if err := app.tokenBroadcaster.Start(); err != nil {
		return fmt.Errorf("failed to start token broadcaster: %v", err)
	}
	defer app.tokenBroadcaster.Close()

	// Initialize email sender
	if smtpEnabled {
		// Determine SMTP security
		var security smtpSecurity
		if cfg.InsecureSMTP {
			security = smtpInsecureDangerous
		} else {
			security = smtpSecure
		}

		// Create email config
		emailConfig := &smtpConfig{
			senderName:       cfg.SMTPSenderName,
			senderEmail:      cfg.SMTPSenderEmail,
			serverHost:       cfg.SMTPServerHost,
			serverPort:       cfg.SMTPServerPort,
			ipVersion:        ipv4,
			domain:           cfg.SMTPDomain,
			security:         security,
			disableKeepAlive: cfg.NoKeepAlive,
			templatesPath:    cfg.EmailTemplatesPath,
		}

		// Load email templates if path is provided
		var templates *template.Template
		if cfg.EmailTemplatesPath != "" {
			var err error
			templates, err = loadEmailTemplates(cfg.EmailTemplatesPath)
			if err != nil {
				return fmt.Errorf("failed to load email templates: %v", err)
			}
			log.Printf("Loaded email templates from %s", cfg.EmailTemplatesPath)
		}

		app.emailSender = &smtpActionsEmailSender{
			config:           emailConfig,
			templates:        templates,
			tokenBroadcaster: app.tokenBroadcaster,
		}

		app.emailSender.m.Lock()
		err := app.emailSender.Start(time.Minute * 5)
		if err != nil {
			app.emailSender.m.Unlock()
			return fmt.Errorf("failed to start email sender: %v", err)
		}
		app.emailSender.m.Unlock()
		defer app.emailSender.Close()
	} else {
		// Create email sender that only broadcasts tokens (no SMTP)
		app.emailSender = &smtpActionsEmailSender{
			tokenBroadcaster: app.tokenBroadcaster,
		}
	}

	// Session expiration
	sessionExpiration := cfg.SessionExpiration
	if sessionExpiration == 0 {
		sessionExpiration = 90 * 24 * time.Hour
	}

	// Create faroe server
	faroeServer := faroe.NewServer(
		app.storage,
		userServerClient,
		&ActionLogger{},
		[]faroe.PasswordHashAlgorithmInterface{userPasswordHashAlgorithm},
		temporaryPasswordHashAlgorithm,
		1,
		faroe.RealClock,
		faroe.AllowAllEmailAddresses,
		app.emailSender,
		faroe.SessionConfigStruct{
			InactivityTimeout:     30 * 24 * time.Hour,
			ActivityCheckInterval: time.Minute,
			Expiration:            sessionExpiration,
		},
	)

	// Start HTTP server
	app.httpServer = &httpServer{
		server:          faroeServer,
		storage:         app.storage,
		enableReset:     cfg.EnableReset,
		corsAllowOrigin: cfg.CORSAllowOrigin,
	}
	app.httpServer.listen(cfg.Port)

	// Start interactive shell if enabled
	app.shell = newInteractiveShell(app.storage)
	if cfg.EnableInteractive {
		app.shell.listen()
	}

	// Wait for errors
	for {
		if smtpEnabled {
			select {
			case serverErr := <-app.httpServer.errChan:
				return serverErr
			case mailErr := <-app.emailSender.errChan:
				return mailErr
			case shellErr := <-app.shell.errChan:
				return shellErr
			}
		} else {
			select {
			case serverErr := <-app.httpServer.errChan:
				return serverErr
			case shellErr := <-app.shell.errChan:
				return shellErr
			}
		}
	}
}
