package config

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/99designs/gqlgen/codegen/config"
	"github.com/nautilus/graphql"
	"github.com/vektah/gqlparser/v2/formatter"
	"gopkg.in/yaml.v3"
)

type contextKey int

const (
	configContextKey contextKey = iota + 1024
)

func NewWithContext(ctx context.Context) (context.Context, error) {
	cfg, err := LoadConfigFromDefaultLocations()
	if err != nil {
		return nil, err
	}
	return WithContext(ctx, cfg), nil
}

func WithContext(ctx context.Context, cfg *Config) context.Context {
	return context.WithValue(ctx, configContextKey, cfg)
}

func FromContext(ctx context.Context) *Config {
	if val := ctx.Value(configContextKey); val != nil {
		return val.(*Config)
	}
	panic("misplaced config in context")
}

type SchemaEndpoint struct {
	URL     string            `yaml:"url"`
	Headers map[string]string `yaml:"headers,omitempty"`
}

type ExtendedConfig struct {
	SchemaEndpoint `yaml:"schema_endpoint"`
	Client         struct {
		config.PackageConfig `yaml:",inline"`
		InterfaceName        string `yaml:"interface_name"`
	} `yaml:"client"`
}

type Config struct {
	ExtendedConfig `yaml:",inline"`
	*config.Config `yaml:",inline"`
}

var cfgFilenames = []string{".gqlgen.yml", "gqlgen.yml", "gqlgen.yaml"}

// LoadConfigFromDefaultLocations looks for a config file in the current directory, and all parent directories
// walking up the tree. The closest config file will be returned.
func LoadConfigFromDefaultLocations() (*Config, error) {
	cfgFile, err := findCfg()
	if err != nil {
		return nil, err
	}

	err = os.Chdir(filepath.Dir(cfgFile))
	if err != nil {
		return nil, fmt.Errorf("unable to enter config dir: %w", err)
	}

	return LoadConfig(cfgFile)
}

// LoadConfig reads the gqlgen.yml config file
func LoadConfig(filename string) (*Config, error) {
	b, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("unable to read config: %w", err)
	}

	return ReadConfig(bytes.NewReader(b))
}

func ReadConfig(cfgFile io.Reader) (*Config, error) {
	cfg := &Config{Config: config.DefaultConfig()}

	dec := yaml.NewDecoder(cfgFile)
	dec.KnownFields(true)

	if err := dec.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("unable to parse config: %w", err)
	}

	if len(cfg.SchemaEndpoint.URL) > 0 {
		remoteSchema, err := graphql.IntrospectRemoteSchema(
			cfg.SchemaEndpoint.URL,
			graphql.IntrospectWithMiddlewares(
				func(r *http.Request) error {
					for k, v := range cfg.SchemaEndpoint.Headers {
						r.Header.Set(k, v)
					}
					return nil
				},
			),
		)

		if err != nil {
			return nil, fmt.Errorf("unable to introspect remote schema: %w", err)
		}

		f, err := os.CreateTemp("", "schema.graphql")
		if err != nil {
			return nil, fmt.Errorf("unable to create schema file: %w", err)
		}

		formatter.NewFormatter(f).FormatSchema(remoteSchema.Schema)
		cfg.SchemaFilename = append(cfg.SchemaFilename, f.Name())
	}

	if err := config.CompleteConfig(cfg.Config); err != nil {
		return nil, err
	}

	if err := cfg.Client.Check(); err != nil {
		return nil, fmt.Errorf("config.client: %w", err)
	}

	return cfg, nil
}

// findCfg searches for the config file in this directory and all parents up the tree
// looking for the closest match
func findCfg() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("unable to get working dir to findCfg: %w", err)
	}

	cfg := findCfgInDir(dir)

	for cfg == "" && dir != filepath.Dir(dir) {
		dir = filepath.Dir(dir)
		cfg = findCfgInDir(dir)
	}

	if cfg == "" {
		return "", os.ErrNotExist
	}

	return cfg, nil
}

func findCfgInDir(dir string) string {
	for _, cfgName := range cfgFilenames {
		path := filepath.Join(dir, cfgName)
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}
