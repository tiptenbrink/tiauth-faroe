package tiauth

import (
	"log"
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
	config        Config
	storage       *storageStruct
	emailSender   *backendEmailSender
	backendClient *BackendClient
	httpServer    *httpServer
	cmdServer     *commandServer
}

// Run starts the tiauth-faroe server with the given configuration.
// This is a blocking call that runs until an error occurs.
//
// Startup sequence:
//  1. Initialize storage
//  2. Create backend HTTP client for Python communication
//  3. Initialize faroe server components
//  4. Start HTTP server
func Run(cfg Config) error {
	app := &App{config: cfg}

	// Initialize storage
	app.storage = newStorage(cfg.DBPath)
	defer app.storage.Close()

	// Create backend HTTP client for Python communication
	app.backendClient = NewBackendClient(cfg.PrivateHost, cfg.UserServerPort)

	// Initialize user action client using backend HTTP
	userServerClient := faroe.NewUserServerClient(app.backendClient)

	// Initialize password hash algorithms
	userPasswordHashAlgorithm := newArgon2id(3, 1024*64, 1)
	temporaryPasswordHashAlgorithm := newArgon2id(3, 1024*16, 1)

	// Create email sender (delegates to Python backend)
	app.emailSender = &backendEmailSender{
		backendClient: app.backendClient,
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
		corsAllowOrigin: cfg.CORSAllowOrigin,
	}
	app.httpServer.listen(cfg.Port)

	// Start command server on private host
	app.cmdServer = &commandServer{storage: app.storage, host: cfg.PrivateHost}
	app.cmdServer.listen(cfg.CommandPort)

	// Wait for either server to error
	select {
	case err := <-app.httpServer.errChan:
		return err
	case err := <-app.cmdServer.errChan:
		return err
	}
}
