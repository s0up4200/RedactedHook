package config

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/fsnotify/fsnotify"
	"github.com/inhies/go-bytesize"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

const EnvPrefix = "REDACTEDHOOK__"

func InitConfig(configPath string) {
	configFile := determineConfigFile(configPath)
	setupViper(configFile)
	readAndUnmarshalConfig()
	watchConfigChanges()
}

func setupViper(configFile string) {
	viper.SetDefault("userid.red_user_id", 0)
	viper.SetDefault("userid.ops_user_id", 0)
	viper.SetDefault("ratio.minratio", 0)
	viper.SetDefault("sizecheck.minsize", "")
	viper.SetDefault("sizecheck.maxsize", "")
	viper.SetDefault("uploaders.uploaders", "")
	viper.SetDefault("uploaders.mode", "")
	viper.SetDefault("record_labels.record_labels", "")

	viper.SetConfigType("toml")
	viper.AutomaticEnv()
	viper.SetEnvPrefix(EnvPrefix[:len(EnvPrefix)-2])
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AllowEmptyEnv(true)
	viper.SetConfigFile(configFile)

	configContent, err := os.ReadFile(configFile)
	if err != nil {
		log.Fatal().Err(err).Msg("Error reading config file")
	}

	expandedConfig := os.ExpandEnv(string(configContent))

	if err := viper.ReadConfig(strings.NewReader(expandedConfig)); err != nil {
		log.Fatal().Err(err).Msg("Error reading config file")
	}
}

func readAndUnmarshalConfig() {
	if err := viper.Unmarshal(&config); err != nil {
		log.Error().Err(err).Msg("Unable to unmarshal config")
	} else {
		parseSizeCheck()
		log.Debug().Msgf("Config file read: %s", viper.ConfigFileUsed())
		configureLogger()
	}
}

func parseSizeCheck() {
	minSizeStr := viper.GetString("sizecheck.minsize")
	if minSizeStr == "" {
		config.ParsedSizes.MinSize = 0
	} else {
		if minSize, err := bytesize.Parse(minSizeStr); err != nil {
			log.Error().Err(err).Msg("Invalid format for MinSize; unable to parse")
		} else {
			config.ParsedSizes.MinSize = minSize
		}
	}

	maxSizeStr := viper.GetString("sizecheck.maxsize")
	if maxSizeStr == "" {
		config.ParsedSizes.MaxSize = 0
	} else {
		if maxSize, err := bytesize.Parse(maxSizeStr); err != nil {
			log.Error().Err(err).Msg("Invalid format for MaxSize; unable to parse")
		} else {
			config.ParsedSizes.MaxSize = maxSize
		}
	}
}

func watchConfigChanges() {
	viper.WatchConfig()
	viper.OnConfigChange(func(e fsnotify.Event) {
		handleConfigChange(e)
	})
}

func handleConfigChange(e fsnotify.Event) {
	oldConfig := config

	if err := viper.ReadInConfig(); err != nil {
		log.Error().Err(err).Msg("Error reading config")
		return
	}
	if err := viper.Unmarshal(&config); err != nil {
		log.Error().Err(err).Msg("Error unmarshalling config")
		return
	}

	parseSizeCheck()
	logConfigChanges(oldConfig, config)

	if oldConfig.Logs.LogLevel != config.Logs.LogLevel {
		configureLogger()
	}
	log.Debug().Msgf("Config file updated: %s", e.Name)
}

func logConfigChanges(oldConfig, newConfig Config) {
	if oldConfig.Server.Host != newConfig.Server.Host {
		log.Debug().Msgf("Server host changed from %s to %s", oldConfig.Server.Host, newConfig.Server.Host)
	}
	if oldConfig.IndexerKeys.REDKey != newConfig.IndexerKeys.REDKey {
		log.Debug().Msg("red_apikey changed")
	}
	if oldConfig.IndexerKeys.OPSKey != newConfig.IndexerKeys.OPSKey {
		log.Debug().Msg("ops_apikey changed")
	}

	if oldConfig.UserIDs.REDUserID != newConfig.UserIDs.REDUserID {
		log.Debug().Msgf("REDUserID changed from %d to %d", oldConfig.UserIDs.REDUserID, newConfig.UserIDs.REDUserID)
	}
	if oldConfig.UserIDs.OPSUserID != newConfig.UserIDs.OPSUserID {
		log.Debug().Msgf("OPSUserID changed from %d to %d", oldConfig.UserIDs.OPSUserID, newConfig.UserIDs.OPSUserID)
	}

	if oldConfig.Ratio.MinRatio != newConfig.Ratio.MinRatio {
		log.Debug().Msgf("MinRatio changed from %f to %f", oldConfig.Ratio.MinRatio, newConfig.Ratio.MinRatio)
	}

	if oldConfig.ParsedSizes.MinSize != newConfig.ParsedSizes.MinSize {
		log.Debug().Msgf("MinSize changed from %s to %s", oldConfig.ParsedSizes.MinSize, newConfig.ParsedSizes.MinSize)
	}

	if oldConfig.ParsedSizes.MaxSize != newConfig.ParsedSizes.MaxSize {
		log.Debug().Msgf("MaxSize changed from %s to %s", oldConfig.ParsedSizes.MaxSize, newConfig.ParsedSizes.MaxSize)
	}

	if oldConfig.Uploaders.Uploaders != newConfig.Uploaders.Uploaders {
		log.Debug().Msgf("Uploaders changed from %s to %s", oldConfig.Uploaders.Uploaders, newConfig.Uploaders.Uploaders)
	}
	if oldConfig.Uploaders.Mode != newConfig.Uploaders.Mode {
		log.Debug().Msgf("Uploader mode changed from %s to %s", oldConfig.Uploaders.Mode, newConfig.Uploaders.Mode)
	}

	if oldConfig.Logs.LogLevel != newConfig.Logs.LogLevel {
		log.Debug().Msgf("Log level changed from %s to %s", oldConfig.Logs.LogLevel, newConfig.Logs.LogLevel)
	}
	if oldConfig.Logs.LogToFile != newConfig.Logs.LogToFile {
		log.Debug().Msgf("LogToFile changed from %t to %t", oldConfig.Logs.LogToFile, newConfig.Logs.LogToFile)
	}
	if oldConfig.Logs.LogFilePath != newConfig.Logs.LogFilePath {
		log.Debug().Msgf("LogFilePath changed from %s to %s", oldConfig.Logs.LogFilePath, newConfig.Logs.LogFilePath)
	}
	if oldConfig.Logs.MaxSize != newConfig.Logs.MaxSize {
		log.Debug().Msgf("Logs MaxSize changed from %d to %d", oldConfig.Logs.MaxSize, newConfig.Logs.MaxSize)
	}
	if oldConfig.Logs.MaxBackups != newConfig.Logs.MaxBackups {
		log.Debug().Msgf("Logs MaxBackups changed from %d to %d", oldConfig.Logs.MaxBackups, newConfig.Logs.MaxBackups)
	}
	if oldConfig.Logs.MaxAge != newConfig.Logs.MaxAge {
		log.Debug().Msgf("Logs MaxAge changed from %d to %d", oldConfig.Logs.MaxAge, newConfig.Logs.MaxAge)
	}
	if oldConfig.Logs.Compress != newConfig.Logs.Compress {
		log.Debug().Msgf("Logs Compress changed from %t to %t", oldConfig.Logs.Compress, newConfig.Logs.Compress)
	}
}

func ValidateConfig() error {
	var validationErrors []string

	apiToken := viper.GetString("authorization.api_token")
	if envToken, exists := os.LookupEnv(EnvPrefix + "API_TOKEN"); exists {
		apiToken = envToken
	}
	if apiToken == "" {
		validationErrors = append(validationErrors, "Authorization API Token is required.")
	}

	redApiKey := viper.GetString("indexer_keys.red_apikey")
	if envRedKey, exists := os.LookupEnv(EnvPrefix + "RED_APIKEY"); exists {
		redApiKey = envRedKey
	}

	opsApiKey := viper.GetString("indexer_keys.ops_apikey")
	if envOpsKey, exists := os.LookupEnv(EnvPrefix + "OPS_APIKEY"); exists {
		opsApiKey = envOpsKey
	}

	if redApiKey == "" && opsApiKey == "" {
		validationErrors = append(validationErrors, "At least one indexer API key (RED or OPS) must be configured")
	}

	host := viper.GetString("server.host")
	if envHost, exists := os.LookupEnv(EnvPrefix + "HOST"); exists {
		host = envHost
	}
	if host == "" {
		validationErrors = append(validationErrors, "Server host is required either in config or as an environment variable.")
	}

	port := viper.GetInt("server.port")
	if envPort, exists := os.LookupEnv(EnvPrefix + "PORT"); exists {
		var err error
		if _, err = fmt.Sscanf(envPort, "%d", &port); err != nil {
			validationErrors = append(validationErrors, "Invalid port number in environment variable")
		}
	}

	if port <= 0 {
		validationErrors = append(validationErrors, "Server port is required either in config or as a positive integer environment variable.")
	}

	if len(validationErrors) > 0 {
		return errors.New(strings.Join(validationErrors, "; "))
	}

	return nil
}
