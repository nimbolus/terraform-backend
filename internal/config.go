package internal

import (
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// SecretEnvOrFile returns the value of an environment variable or the content of a file.
// If both are set, the file content is returned.
func SecretEnvOrFile(envName, envFileName string) (string, error) {
	env := viper.GetString(envName)
	envFile := viper.GetString(envFileName)

	if len(envFile) > 0 {
		if len(env) > 0 {
			log.Warnf("both %s and %s are set, using %s", envName, envFileName, envFileName)
		}

		envFileContent, err := os.ReadFile(envFile)
		if err != nil {
			return "", fmt.Errorf("reading %s: %w", envFile, err)
		}

		return string(envFileContent), nil
	}

	return env, nil
}
