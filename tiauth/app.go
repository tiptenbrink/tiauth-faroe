package tiauth

import (
	"fmt"
	"log"
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
		return fmt.Errorf("UserActionInvocationURL is required")
	}
	if cfg.SMTPSenderEmail == "" {
		return fmt.Errorf("SMTPSenderEmail is required")
	}
	if cfg.SMTPServerHost == "" {
		return fmt.Errorf("SMTPServerHost is required")
	}
	if cfg.SMTPServerPort == "" {
		return fmt.Errorf("SMTPServerPort is required")
	}
	if cfg.SMTPDomain == "" {
		return fmt.Errorf("SMTPDomain is required")
	}

	// Initialize storage
	app.storage = newStorage(cfg.DBPath)
	defer app.storage.Close()

	// Initialize user action client
	userActionInvocationClient := newUserActionInvocationClient(cfg.UserActionInvocationURL)
	userServerClient := faroe.NewUserServerClient(userActionInvocationClient)

	// Initialize password hash algorithms
	userPasswordHashAlgorithm := newArgon2id(3, 1024*64, 1)
	temporaryPasswordHashAlgorithm := newArgon2id(3, 1024*16, 1)

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

	// Initialize token broadcaster
	app.tokenBroadcaster = NewTokenBroadcaster(cfg.TokenSocketPath)
	if err := app.tokenBroadcaster.Start(); err != nil {
		return fmt.Errorf("failed to start token broadcaster: %v", err)
	}
	defer app.tokenBroadcaster.Close()

	// Initialize email sender
	app.emailSender = &smtpActionsEmailSender{
		config:           emailConfig,
		templates:        templates,
		tokenBroadcaster: app.tokenBroadcaster,
	}

	if !cfg.NoSMTPInit {
		app.emailSender.m.Lock()
		err := app.emailSender.Start(time.Minute * 5)
		if err != nil {
			app.emailSender.m.Unlock()
			return fmt.Errorf("failed to start email sender: %v", err)
		}
		app.emailSender.m.Unlock()
	}
	defer app.emailSender.Close()

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
		select {
		case serverErr := <-app.httpServer.errChan:
			return serverErr
		case mailErr := <-app.emailSender.errChan:
			return mailErr
		case shellErr := <-app.shell.errChan:
			return shellErr
		}
	}
}
