package config

import (
	"bufio"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
)

const defaultConfigName = ".hdtconfig"

type R2 struct {
	Endpoint        string
	AccessKeyID     string
	SecretAccessKey string
	Bucket          string
	Prefix          string
}

type Config struct {
	ConfigFile string
	Paths      []string
	R2         R2
}

func Load() (Config, error) {
	return LoadForBackup(true)
}

// Restore/list do not require a source configuration on a new computer.
func LoadForBackup(requirePaths bool) (Config, error) {
	if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		return Config{}, fmt.Errorf("carregar .env: %w", err)
	}
	configDir, err := os.UserConfigDir()
	if err != nil {
		return Config{}, fmt.Errorf("diretório de configuração: %w", err)
	}
	if err := godotenv.Load(filepath.Join(configDir, "holdotfiles", ".env")); err != nil && !errors.Is(err, os.ErrNotExist) {
		return Config{}, fmt.Errorf("carregar configuração global: %w", err)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return Config{}, fmt.Errorf("obter diretório pessoal: %w", err)
	}

	configFile := strings.TrimSpace(os.Getenv("HOLDOTFILES_CONFIG"))
	if configFile == "" {
		configFile = filepath.Join(home, defaultConfigName)
	} else {
		configFile, err = ExpandPath(configFile, home)
		if err != nil {
			return Config{}, fmt.Errorf("caminho de configuração: %w", err)
		}
	}

	var paths []string
	if requirePaths {
		paths, err = ReadPaths(configFile, home)
		if err != nil {
			return Config{}, err
		}
	}

	accountID := strings.TrimSpace(os.Getenv("R2_ACCOUNT_ID"))
	endpoint := strings.TrimSpace(os.Getenv("R2_ENDPOINT"))
	if endpoint == "" && accountID != "" {
		endpoint = fmt.Sprintf("https://%s.r2.cloudflarestorage.com", accountID)
	}

	r2 := R2{
		Endpoint:        endpoint,
		AccessKeyID:     strings.TrimSpace(os.Getenv("R2_ACCESS_KEY_ID")),
		SecretAccessKey: strings.TrimSpace(os.Getenv("R2_SECRET_ACCESS_KEY")),
		Bucket:          strings.TrimSpace(os.Getenv("R2_BUCKET")),
		Prefix:          strings.Trim(strings.TrimSpace(os.Getenv("R2_PREFIX")), "/"),
	}
	if r2.Prefix == "" {
		r2.Prefix = defaultPrefix()
	}
	if err := validateR2(r2); err != nil {
		return Config{}, err
	}

	return Config{ConfigFile: configFile, Paths: paths, R2: r2}, nil
}

func ReadPaths(filename, home string) ([]string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("abrir %s: %w", filename, err)
	}
	defer file.Close()

	seen := make(map[string]struct{})
	var paths []string
	scanner := bufio.NewScanner(file)
	for lineNumber := 1; scanner.Scan(); lineNumber++ {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		expanded, err := ExpandPath(line, home)
		if err != nil {
			return nil, fmt.Errorf("%s, linha %d: %w", filename, lineNumber, err)
		}
		if _, ok := seen[expanded]; ok {
			continue
		}
		seen[expanded] = struct{}{}
		paths = append(paths, expanded)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("ler %s: %w", filename, err)
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("%s não contém nenhum caminho", filename)
	}
	return paths, nil
}

func ExpandPath(value, home string) (string, error) {
	value = strings.TrimSpace(value)
	switch {
	case value == "~":
		value = home
	case strings.HasPrefix(value, "~/") || strings.HasPrefix(value, `~\`):
		value = filepath.Join(home, value[2:])
	case strings.HasPrefix(value, "~"):
		return "", fmt.Errorf("usuários alternativos não são suportados: %q", value)
	}

	absolute, err := filepath.Abs(value)
	if err != nil {
		return "", err
	}
	return filepath.Clean(absolute), nil
}

func validateR2(cfg R2) error {
	missing := make([]string, 0, 4)
	for _, variable := range []struct {
		name  string
		value string
	}{
		{"R2_ENDPOINT ou R2_ACCOUNT_ID", cfg.Endpoint},
		{"R2_ACCESS_KEY_ID", cfg.AccessKeyID},
		{"R2_SECRET_ACCESS_KEY", cfg.SecretAccessKey},
		{"R2_BUCKET", cfg.Bucket},
	} {
		if variable.value == "" {
			missing = append(missing, variable.name)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("variáveis obrigatórias ausentes: %s", strings.Join(missing, ", "))
	}
	parsed, err := url.ParseRequestURI(cfg.Endpoint)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return fmt.Errorf("R2_ENDPOINT inválido: %q", cfg.Endpoint)
	}
	return nil
}

func defaultPrefix() string {
	hostname, err := os.Hostname()
	if err != nil || strings.TrimSpace(hostname) == "" {
		return "default"
	}
	hostname = strings.ToLower(hostname)
	return strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' || r == '_' {
			return r
		}
		return '-'
	}, hostname)
}
