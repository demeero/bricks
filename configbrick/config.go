package configbrick

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/kelseyhightower/envconfig"
)

func LoadConfig(cfg any, log bool) {
	envconfig.MustProcess("", cfg)
	if !log {
		return
	}
	b, err := json.Marshal(cfg)
	if err != nil {
		slog.Error("failed marshal config", slog.Any("err", err))
		return
	}
	slog.Info("parsed config", slog.String("config", string(b)))
}

// AppMeta represents the application metadata.
type AppMeta struct {
	Env              string `default:"local" json:"env"`
	ServiceName      string `default:"unknown-service-name" split_words:"true" json:"service_name"`
	ServiceNamespace string `default:"unknown-service-namespace" split_words:"true" json:"service_namespace"`
}

// Log represents the log configuration.
type Log struct {
	Level     string `default:"debug" json:"level"`
	AddSource bool   `split_words:"true" json:"add_source"`
	JSON      bool   `json:"json"`
}

// HTTP represents the HTTP server configuration.
type HTTP struct {
	AccessLog         bool          `split_words:"true" json:"access_log"`
	AccessLogLevel    string        `default:"debug" split_words:"true" json:"access_log_level"`
	ReadHeaderTimeout time.Duration `default:"10s" split_words:"true" json:"read_header_timeout"`
	ReadTimeout       time.Duration `default:"30s" split_words:"true" json:"read_timeout"`
	WriteTimeout      time.Duration `default:"30s" split_words:"true" json:"write_timeout"`
	Port              int           `default:"8088" json:"port"`
	ShutdownTimeout   time.Duration `default:"10s" split_words:"true" json:"shutdown_timeout"`
}

// Redis represents the Redis configuration.
type Redis struct {
	Addr     string `default:"localhost:6379" json:"addr"`
	Password string `json:"-"`
	DB       int    `json:"db"`
}

// Mongo represents the MongoDB configuration.
type Mongo struct {
	// DBName is the name of the database to use.
	DBName string `split_words:"true" json:"db_name"`
	// URI is the MongoDB connection URI.
	URI string   `default:"mongodb://localhost:27017"`
	Log MongoLog `json:"log"`
	// InitialConnectTimeout is the time to wait for the initial connection to the database during app setup.
	InitialConnectTimeout time.Duration `default:"30s" split_words:"true" json:"initial_connect_timeout"`
}

// MongoLog represents the MongoDB client logging configuration.
type MongoLog struct {
	Commands bool `json:"commands"`
	Result   bool `json:"result"`
	Fails    bool `json:"fails"`
}

// Cassandra represents the Cassandra configuration.
type Cassandra struct {
	Host     string `default:"localhost:9042" json:"host"`
	Keyspace string `default:"chat" json:"keyspace"`
	Log      bool   `json:"log"`
}

// OTEL represents the OpenTelemetry configuration.
type OTEL struct {
	// Meter represents the OpenTelemetry meter configuration.
	Meter OTLP `json:"meter"`
	// Trace represents the OpenTelemetry trace configuration.
	Trace OTLP `json:"trace"`
}

type OTLP struct {
	// Enabled indicates if the exporter is enabled.
	Enabled bool `json:"enabled"`
	// Endpoint is the target to which the exporter is going to send data.
	Endpoint string `json:"endpoint"`
	// Insecure indicates if the exporter should skip TLS verification.
	Insecure bool `json:"insecure"`
	// PathPrefix is the path prefix to use for HTTP requests.
	PathPrefix string `json:"path_prefix"`
	// Username is the username to use for HTTP Basic Auth.
	Username string `json:"-"`
	// Password is the password to use for HTTP Basic Auth.
	Password string `json:"-"`
}

// BasicAuthHeader returns the HTTP Basic Auth header.
func (cfg OTLP) BasicAuthHeader() map[string]string {
	auth := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", cfg.Username, cfg.Password)))
	return map[string]string{"Authorization": "Basic " + auth}
}

// UserPassword represents the user password configuration.
type UserPassword struct {
	// MinLen is the minimum length of the password.
	MinLen int `default:"8" split_words:"true" json:"min_len"`
	// MaxLen is the maximum length of the password.
	MaxLen int `default:"64" split_words:"true" json:"max_len"`
	// MustHaveNum indicates if the password must have at least one number.
	MustHaveNum bool `default:"true" split_words:"true" json:"must_have_num"`
	// MustHaveUpper indicates if the password must have at least one uppercase letter.
	MustHaveUpper bool `default:"true" split_words:"true" json:"must_have_upper"`
	// MustHaveLower indicates if the password must have at least one lowercase letter.
	MustHaveLower bool `default:"true" split_words:"true" json:"must_have_lower"`
	// MustHaveSpecial indicates if the password must have at least one special character.
	MustHaveSpecial bool `default:"true" split_words:"true" json:"must_have_special"`
	// BCryptCost is the cost of the bcrypt algorithm.
	BCryptCost int `default:"10" json:"bcrypt_cost"`
}
