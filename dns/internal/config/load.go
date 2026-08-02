package config

import (
	"errors"
	"fmt"
	"io"
	"os"

	"gopkg.in/yaml.v3"
)

// Load reads, decodes, and validates a YAML configuration file.
func Load(path string) (Config, error) {
	if path == "" {
		return Config{}, errors.New("configuration path must not be empty")
	}

	file, err := os.Open(path)
	if err != nil {
		return Config{}, fmt.Errorf("open configuration file %q: %w", path, err)
	}
	defer file.Close()

	cfg := Default()

	decoder := yaml.NewDecoder(file)
	decoder.KnownFields(true)

	if err := decoder.Decode(&cfg); err != nil {
		return Config{}, fmt.Errorf("decode configuration file %q: %w", path, err)
	}

	var extraDocument any
	err = decoder.Decode(&extraDocument)

	switch {
	case errors.Is(err, io.EOF):
		// Expected: the file contains exactly one YAML document.
	case err != nil:
		return Config{}, fmt.Errorf(
			"decode additional YAML content in %q: %w",
			path,
			err,
		)
	default:
		return Config{}, fmt.Errorf(
			"configuration file %q must contain exactly one YAML document",
			path,
		)
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, fmt.Errorf("validate configuration file %q: %w", path, err)
	}

	return cfg, nil
}
