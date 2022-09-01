package main

import (
	"flag"
	"fmt"
	"net/http"
	"time"

	_ "expvar"

	_ "github.com/lib/pq"

	"github.com/cyverse-de/go-mod/cfg"
	"github.com/cyverse-de/go-mod/logging"
	"github.com/cyverse-de/go-mod/protobufjson"
	"github.com/cyverse-de/monitoring-api/checkconfigs"
	"github.com/cyverse-de/monitoring-api/checkresults"
	"github.com/cyverse-de/monitoring-api/checktypes"
	"github.com/cyverse-de/monitoring-api/natsconn"
	"github.com/cyverse-de/p/go/monitoring"
	"github.com/jmoiron/sqlx"
	"github.com/knadh/koanf"
	"github.com/nats-io/nats.go"
	"github.com/sirupsen/logrus"
)

const serviceName = "monitoring-agent"

var log = logging.Log.WithFields(logrus.Fields{"service": serviceName})

func initNATS(c *koanf.Koanf, envPrefix *string) (*natsconn.Connector, error) {
	natsCluster := c.String("nats.cluster")
	if natsCluster == "" {
		log.Fatalf("The %sNATS_CLUSTER environment variable or nats.cluster configuration value must be set", *envPrefix)
	}

	tlsCert := c.String("nats.tls.cert")
	if tlsCert == "" {
		log.Fatalf("The %sNATS_TLS_CERT environment variable or nats.tlscert configuration value must be set", *envPrefix)
	}

	tlsKey := c.String("nats.tls.key")
	if tlsKey == "" {
		log.Fatalf("The %sNATS_TLS_KEY environment variable or nats.tlskey configuration value must be set", *envPrefix)
	}

	caCert := c.String("nats.tls.ca.cert")
	if caCert == "" {
		log.Fatalf("The %sNATS_TLS_CA_CERT environment variable or nats.cacert configuration value must be set", *envPrefix)
	}

	credsPath := c.String("nats.creds.path")
	if credsPath == "" {
		log.Fatalf("The %sNATS_CREDS_PATH environment variable or nats.creds configuration value must be set", *envPrefix)
	}

	maxReconnects := c.Int("nats.reconnects.max")
	reconnectWait := c.Int("nats.reconnects.wait")

	natsSubject := c.String("nats.basesubject")
	if natsSubject == "" {
		log.Fatalf("The %sNATS_BASESUBJECT environment variable or nats.basesubject configuration value must be set", *envPrefix)
	}

	natsQueue := c.String("nats.basequeue")
	if natsQueue == "" {
		log.Fatalf("The %sNATS_BASEQUEUE environment variable or nats.basequeue configuration value must be set", *envPrefix)
	}

	log.Infof("nats.cluster is set to '%s'", natsCluster)
	log.Infof("NATS TLS cert file is %s", tlsCert)
	log.Infof("NATS TLS key file is %s", tlsKey)
	log.Infof("NATS CA cert file is %s", caCert)
	log.Infof("NATS creds file is %s", credsPath)
	log.Infof("NATS max reconnects is %d", maxReconnects)
	log.Infof("NATS reonnect wait is %d", reconnectWait)

	natsConn, err := natsconn.NewConnector(&natsconn.ConnectorSettings{
		BaseSubject:   natsSubject,
		BaseQueue:     natsQueue,
		NATSCluster:   natsCluster,
		CredsPath:     credsPath,
		TLSKeyPath:    tlsKey,
		TLSCertPath:   tlsCert,
		CAPath:        caCert,
		MaxReconnects: maxReconnects,
		ReconnectWait: reconnectWait,
	})
	if err != nil {
		log.Fatal(err)
	}

	return natsConn, err
}

type App struct {
	dbconn            *sqlx.DB
	natsConn          *natsconn.Connector
	checkResulter     *checkresults.CheckResulter
	checkConfigurator *checkconfigs.CheckConfigurator
	checkTyper        *checktypes.CheckTyper
}

func NewApp(dbconn *sqlx.DB, natsConn *natsconn.Connector) *App {
	return &App{
		dbconn:            dbconn,
		natsConn:          natsConn,
		checkResulter:     checkresults.New(dbconn),
		checkConfigurator: checkconfigs.New(dbconn),
		checkTyper:        checktypes.New(dbconn),
	}
}
func (a *App) DNSCheckMsgHandler(subject, reply string, request *monitoring.DNSCheckResult) {
	var err error

	result := &checkresults.CheckResult{}
	result.Node = request.Node
	result.DateReceived = time.Now()
	result.DateSent, err = time.Parse("2006-01-02 15:04:05.999999999 -0700 MST", request.DateSent)
	if err != nil {
		log.Error(err)
		return
	}

	lookups := request.Lookups
	success := true
	for _, l := range lookups {
		if l.Error != "" {
			result.Error = result.Error + l.Error + "\n"
			success = false
		}
	}
	result.Successful = success
}

func (a *App) HeartbeatCheckMsgHandler(subject, reply string, request *monitoring.Heartbeat) {
	var err error

	result := &checkresults.CheckResult{}
	result.Node = request.Node
	result.DateReceived = time.Now()
	result.DateSent, err = time.Parse("2006-01-02 15:04:05.999999999 -0700 MST", request.DateSent)
	if err != nil {
		log.Error(err)
		return
	}
	result.Successful = true
}

func main() {
	var (
		err error
		c   *koanf.Koanf

		configPath = flag.String("config", cfg.DefaultConfigPath, "The path to the config file")
		dotEnvPath = flag.String("dotenv-path", cfg.DefaultDotEnvPath, "The path to the env file to load")
		logLevel   = flag.String("log-level", "info", "One of trace, debug, info, warn, error, fatal, or panic.")
		envPrefix  = flag.String("env-prefix", cfg.DefaultEnvPrefix, "The prefix to look for when setting configuration setting in environment variables")
		varsPort   = flag.Int("vars-port", 60000, "The port to listen on for requests to /debug/vars")
	)
	flag.Parse()

	logging.SetupLogging(*logLevel)

	nats.RegisterEncoder("protojson", protobufjson.NewCodec(protobufjson.WithEmitUnpopulated()))

	c, err = cfg.Init(&cfg.Settings{
		EnvPrefix:   *envPrefix,
		ConfigPath:  *configPath,
		DotEnvPath:  *dotEnvPath,
		StrictMerge: false,
		FileType:    cfg.YAML,
	})
	if err != nil {
		log.Fatal(err)
	}
	log.Infof("Done reading config from %s", *configPath)

	dbURI := c.String("db.uri")
	if dbURI == "" {
		log.Fatal("db.uri must be set in the configuration file")
	}

	natsConn, err := initNATS(c, envPrefix)
	if err != nil {
		log.Fatal(err)
	}

	dbconn := sqlx.MustConnect("postgres", dbURI)
	log.Info("done connecting to the database")
	dbconn.SetMaxOpenConns(10)
	dbconn.SetConnMaxIdleTime(time.Minute)

	app := NewApp(dbconn, natsConn)

	pingSubject, pingQueue, err := natsConn.Subscribe("ping", func(m *nats.Msg) {
		log.Info("ping message received")
		err := m.Respond([]byte("pong"))
		if err != nil {
			log.Error(err)
		}
	})
	if err != nil {
		log.Fatal(err)
	}
	log.Infof("subscribed to %s on queue %s via NATS", pingSubject, pingQueue)

	dnsSubject, dnsQueue, err := natsConn.Subscribe("dns", app.DNSCheckMsgHandler)
	if err != nil {
		log.Fatal(err)
	}
	log.Infof("subscribed to %s on queue %s via NATS", dnsSubject, dnsQueue)

	hbSubject, hbQueue, err := natsConn.Subscribe("heartbeat", app.HeartbeatCheckMsgHandler)
	if err != nil {
		log.Fatal(err)
	}
	log.Infof("subscribed to %s on queue %s via NATS", hbSubject, hbQueue)

	portStr := fmt.Sprintf(":%d", *varsPort)
	if err = http.ListenAndServe(portStr, nil); err != nil {
		log.Fatal(err)
	}
}
