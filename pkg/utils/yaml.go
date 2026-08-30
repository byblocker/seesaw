package utils

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

func DebugYAML(v any) {
	out, err := yaml.Marshal(v)
	if err != nil {
		fmt.Println("Error marshalling YAML:", err)
		return
	}

	fmt.Println(string(out))
}
