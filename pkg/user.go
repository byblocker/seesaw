package pkg

import (
	"fmt"
	"io"
	"seesaw/pkg/models"

	"gopkg.in/yaml.v3"
)

func ParseUserConfig(file io.Reader) (*models.UserConfig, error) {
	config := &models.UserConfig{}
	decoder := yaml.NewDecoder(file)
	decoder.KnownFields(true)
	if err := decoder.Decode(config); err != nil {
		return nil, fmt.Errorf("failed to parse user config: %w", err)
	}

	var extraDocument any
	if err := decoder.Decode(&extraDocument); err != io.EOF {
		if err != nil {
			return nil, fmt.Errorf("failed to parse user config: %w", err)
		}
		return nil, fmt.Errorf("user config must contain exactly one YAML document")
	}

	if err := config.Tax.Validate(); err != nil {
		return nil, err
	}
	return config, nil
}
