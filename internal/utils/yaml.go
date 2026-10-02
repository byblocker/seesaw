package utils

import "gopkg.in/yaml.v3"

func YAMLToStruct[T any](data []byte) (*T, error) {
	var output T
	if err := yaml.Unmarshal(data, &output); err != nil {
		return nil, err
	}

	return &output, nil
}
