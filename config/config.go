package config

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/ardanlabs/conf"
	"github.com/joho/godotenv"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const (
	DefaultConfigPath = "config/config.toml"
	DefaultEnvPath    = "config/.env"
	ConfigFileName    = "config.toml"
	ServiceName       = "ssi-service"
	ConfigExtension   = ".toml"

	DefaultServiceEndpoint = "http://localhost:8080"

	KeystorePassword EnvironmentVariable = "KEYSTORE_PASSWORD"
	DBPassword       EnvironmentVariable = "DB_PASSWORD"
)

type EnvironmentVariable string

type SSIServiceConfig struct {
	conf.Version
	Server   ServerConfig   `toml:"server"`
	Services ServicesConfig `toml:"services"`
}

// ServerConfig represents configurable properties for the HTTP server
type ServerConfig struct {
	APIHost             string        `toml:"api_host" conf:"default:0.0.0.0:3000"`
	DebugHost           string        `toml:"debug_host" conf:"default:0.0.0.0:4000"`
	JagerHost           string        `toml:"jager_host" conf:"http://jaeger:14268/api/traces"`
	JagerEnabled        bool          `toml:"jager_enabled" conf:"default:false"`
	ReadTimeout         time.Duration `toml:"read_timeout" conf:"default:5s"`
	WriteTimeout        time.Duration `toml:"write_timeout" conf:"default:5s"`
	ShutdownTimeout     time.Duration `toml:"shutdown_timeout" conf:"default:5s"`
	LogLocation         string        `toml:"log_location" conf:"default:log"`
	LogLevel            string        `toml:"log_level" conf:"default:debug"`
	EnableSchemaCaching bool          `toml:"enable_schema_caching" conf:"default:true"`
	EnableAllowAllCORS  bool          `toml:"enable_allow_all_cors" conf:"default:false"`
}

type IssuingServiceConfig struct {
	*BaseServiceConfig
}

func (s *IssuingServiceConfig) IsEmpty() bool {
	if s == nil {
		return true
	}
	return reflect.DeepEqual(s, &IssuingServiceConfig{})
}

// ServicesConfig represents configurable properties for the components of the SSI Service
type ServicesConfig struct {
	// at present, it is assumed that a single storage provider works for all services
	// in the future it may make sense to have per-service storage providers (e.g. mysql for one service,
	// mongo for another)
	StorageProvider string      `toml:"storage"`
	StorageOption   interface{} `toml:"storage_option"`
	ServiceEndpoint string      `toml:"service_endpoint"`

	// Embed all service-specific configs here. The order matters: from which should be instantiated first, to last
	KeyStoreConfig       KeyStoreServiceConfig     `toml:"keystore,omitempty"`
	DIDConfig            DIDServiceConfig          `toml:"did,omitempty"`
	IssuingServiceConfig IssuingServiceConfig      `toml:"issuing,omitempty"`
	SchemaConfig         SchemaServiceConfig       `toml:"schema,omitempty"`
	CredentialConfig     CredentialServiceConfig   `toml:"credential,omitempty"`
	ManifestConfig       ManifestServiceConfig     `toml:"manifest,omitempty"`
	PresentationConfig   PresentationServiceConfig `toml:"presentation,omitempty"`
	WebhookConfig        WebhookServiceConfig      `toml:"webhook,omitempty"`
}

// BaseServiceConfig represents configurable properties for a specific component of the SSI Service
// Can be wrapped and extended for any specific service config
type BaseServiceConfig struct {
	Name            string `toml:"name"`
	ServiceEndpoint string `toml:"service_endpoint"`
}

type KeyStoreServiceConfig struct {
	*BaseServiceConfig
	// Service key password. Used by a KDF whose key is used by a symmetric cypher for key encryption.
	// The password is salted before usage.
	ServiceKeyPassword string `toml:"password"`
}

func (k *KeyStoreServiceConfig) IsEmpty() bool {
	if k == nil {
		return true
	}
	return reflect.DeepEqual(k, &KeyStoreServiceConfig{})
}

type DIDServiceConfig struct {
	*BaseServiceConfig
	Methods                  []string `toml:"methods"`
	LocalResolutionMethods   []string `toml:"local_resolution_methods"`
	UniversalResolverURL     string   `toml:"universal_resolver_url"`
	UniversalResolverMethods []string `toml:"universal_resolver_methods"`
	IONResolverURL           string   `toml:"ion_resolver_url"`
}

func (d *DIDServiceConfig) IsEmpty() bool {
	if d == nil {
		return true
	}
	return reflect.DeepEqual(d, &DIDServiceConfig{})
}

type SchemaServiceConfig struct {
	*BaseServiceConfig
}

func (s *SchemaServiceConfig) IsEmpty() bool {
	if s == nil {
		return true
	}
	return reflect.DeepEqual(s, &SchemaServiceConfig{})
}

type CredentialServiceConfig struct {
	*BaseServiceConfig

	// TODO(gabe) supported key and signature types
}

func (c *CredentialServiceConfig) IsEmpty() bool {
	if c == nil {
		return true
	}
	return reflect.DeepEqual(c, &CredentialServiceConfig{})
}

type ManifestServiceConfig struct {
	*BaseServiceConfig
}

func (m *ManifestServiceConfig) IsEmpty() bool {
	if m == nil {
		return true
	}
	return reflect.DeepEqual(m, &ManifestServiceConfig{})
}

type PresentationServiceConfig struct {
	*BaseServiceConfig
}

func (p *PresentationServiceConfig) IsEmpty() bool {
	if p == nil {
		return true
	}
	return reflect.DeepEqual(p, &PresentationServiceConfig{})
}

type WebhookServiceConfig struct {
	*BaseServiceConfig
}

func (p *WebhookServiceConfig) IsEmpty() bool {
	if p == nil {
		return true
	}
	return reflect.DeepEqual(p, &WebhookServiceConfig{})
}

// LoadConfig attempts to load a TOML config file from the given path, and coerce it into our object model.
// Before loading, defaults are applied on certain properties, which are overwritten if specified in the TOML file.
func LoadConfig(path string) (*SSIServiceConfig, error) {
	loadDefaultConfig, err := checkValidConfigPath(path)
	if err != nil {
		return nil, errors.Wrap(err, "validate config path")
	}

	// create the config object
	var config SSIServiceConfig

	if err := parseAndApplyDefaults(config); err != nil {
		return nil, errors.Wrap(err, "parse and apply defaults")
	}

	if loadDefaultConfig {
		loadDefaultServicesConfig(&config)
	} else {
		if err := loadTOMLConfig(path, &config); err != nil {
			return nil, errors.Wrap(err, "load toml config")
		}
	}

	if err := applyEnvVariables(&config); err != nil {
		return nil, errors.Wrap(err, "apply env variables")
	}

	return &config, nil
}

func checkValidConfigPath(path string) (bool, error) {
	// no path, load default config
	defaultConfig := false
	if path == "" {
		logrus.Info("no config path provided, loading default config...")
		defaultConfig = true
	} else if filepath.Ext(path) != ConfigExtension {
		return false, fmt.Errorf("path<%s> did not match the expected TOML format", path)
	}

	return defaultConfig, nil
}

func parseAndApplyDefaults(config SSIServiceConfig) error {
	// parse and apply defaults
	if err := conf.Parse(os.Args[1:], ServiceName, &config); err != nil {
		switch {
		case errors.Is(err, conf.ErrHelpWanted):
			usage, err := conf.Usage(ServiceName, &config)
			if err != nil {
				return errors.Wrap(err, "parsing config")
			}
			fmt.Println(usage)

			return nil

		case errors.Is(err, conf.ErrVersionWanted):
			version, err := conf.VersionString(ServiceName, &config)
			if err != nil {
				return errors.Wrap(err, "generating config version")
			}

			fmt.Println(version)
			return nil
		}

		return errors.Wrap(err, "parsing config")
	}

	return nil
}

func loadDefaultServicesConfig(config *SSIServiceConfig) {
	servicesConfig := ServicesConfig{
		StorageProvider: "bolt",
		ServiceEndpoint: DefaultServiceEndpoint,
		KeyStoreConfig: KeyStoreServiceConfig{
			BaseServiceConfig:  &BaseServiceConfig{Name: "keystore"},
			ServiceKeyPassword: "default-password",
		},
		DIDConfig: DIDServiceConfig{
			BaseServiceConfig:      &BaseServiceConfig{Name: "did"},
			Methods:                []string{"key", "web"},
			LocalResolutionMethods: []string{"key", "peer", "web", "pkh"},
		},
		SchemaConfig: SchemaServiceConfig{
			BaseServiceConfig: &BaseServiceConfig{Name: "schema"},
		},
		CredentialConfig: CredentialServiceConfig{
			BaseServiceConfig: &BaseServiceConfig{Name: "credential", ServiceEndpoint: DefaultServiceEndpoint},
		},
		ManifestConfig: ManifestServiceConfig{
			BaseServiceConfig: &BaseServiceConfig{Name: "manifest"},
		},
		PresentationConfig: PresentationServiceConfig{
			BaseServiceConfig: &BaseServiceConfig{Name: "presentation"},
		},
		IssuingServiceConfig: IssuingServiceConfig{
			BaseServiceConfig: &BaseServiceConfig{Name: "issuing"},
		},
		WebhookConfig: WebhookServiceConfig{
			BaseServiceConfig: &BaseServiceConfig{Name: "webhook"},
		},
	}

	config.Services = servicesConfig
}

func loadTOMLConfig(path string, config *SSIServiceConfig) error {
	// load from TOML file
	if _, err := toml.DecodeFile(path, &config); err != nil {
		return errors.Wrapf(err, "could not load config: %s", path)
	}

	// apply defaults if not included in toml file
	if config.Services.CredentialConfig.BaseServiceConfig.ServiceEndpoint == "" {
		config.Services.CredentialConfig.BaseServiceConfig.ServiceEndpoint = config.Services.ServiceEndpoint
	}

	return nil
}

func applyEnvVariables(config *SSIServiceConfig) error {
	if err := godotenv.Load(DefaultEnvPath); err != nil {

		// The error indicates that the file or directory does not exist.
		if os.IsNotExist(err) {
			logrus.Info("no .env file found, skipping apply env variables...")
			return nil
		}

		return errors.Wrap(err, "dotenv parsing")
	}

	keystorePassword, present := os.LookupEnv(string(KeystorePassword))

	if present {
		config.Services.KeyStoreConfig.ServiceKeyPassword = keystorePassword
	}

	dbPassword, present := os.LookupEnv(string(DBPassword))

	if present {
		if config.Services.StorageOption == nil {
			config.Services.StorageOption = make(map[string]interface{})
		}

		storageOptionMap, ok := config.Services.StorageOption.(map[string]interface{})
		if !ok {
			return errors.New("storage option must be of type map[string]interface{}")
		}

		storageOptionMap["password"] = dbPassword
		config.Services.StorageOption = storageOptionMap
	}

	return nil
}
