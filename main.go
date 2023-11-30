package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/hashicorp/vault/api"
	"github.com/mitchellh/go-homedir"
	"github.com/sirupsen/logrus"

	"github.com/Luzifer/go_helpers/v2/env"
	"github.com/Luzifer/rconfig/v2"
)

var (
	cfg = struct {
		AppRoleAuth struct {
			RoleID   string `flag:"vault-role-id" env:"VAULT_ROLE_ID" default:"" description:"ID of the role to use"`
			SecretID string `flag:"vault-secret-id" env:"VAULT_SECRET_ID" default:"" description:"Corresponding secret ID to the role"`
		}
		Export    bool   `flag:"export,e" default:"false" description:"Show export statements instead of running the command specified"`
		LogLevel  string `flag:"log-level" default:"info" description:"Verbosity of logs to use (debug, info, warning, error, ...)"`
		Obfuscate string `flag:"obfuscate,o" default:"asterisk" description:"Type of obfuscation (none, asterisk, hash, name)"`
		TokenAuth struct {
			Token string `flag:"vault-token" env:"VAULT_TOKEN" vardefault:"vault-token" description:"Specify a token to use instead of app-id auth"`
		}
		Transform      []string `flag:"transform,t" default:"" description:"Translates keys to different names (oldkey=newkey)"`
		TransformSet   []string `flag:"transform-set" default:"" description:"Apply predefined transform sets (Available: STS)"`
		VaultAddress   string   `flag:"vault-addr" env:"VAULT_ADDR" default:"https://127.0.0.1:8200" description:"Vault API address"`
		VaultKeys      []string `flag:"key,k" default:"" description:"Keys to read and use for environment variables"`
		VersionAndExit bool     `flag:"version" default:"false" description:"Print program version and exit"`
	}{}
	version = "dev"
)

func vaultTokenFromDisk() string {
	vf, err := homedir.Expand("~/.vault-token")
	if err != nil {
		return ""
	}

	data, err := os.ReadFile(vf) //#nosec:G304 // Variable is "safe"
	if err != nil {
		return ""
	}

	return string(data)
}

func initApp() (err error) {
	rconfig.SetVariableDefaults(map[string]string{
		"vault-token": vaultTokenFromDisk(),
	})
	if err = rconfig.Parse(&cfg); err != nil {
		return fmt.Errorf("parsing flags: %w", err)
	}

	ll, err := logrus.ParseLevel(cfg.LogLevel)
	if err != nil {
		return fmt.Errorf("parsing log-level: %w", err)
	}
	logrus.SetLevel(ll)

	if len(cfg.VaultKeys) == 0 || (len(cfg.VaultKeys) == 1 && cfg.VaultKeys[0] == "") {
		return fmt.Errorf("no --key parameters specified")
	}

	if !cfg.Export && len(rconfig.Args()) == 1 {
		return fmt.Errorf("no command specified")
	}

	return nil
}

//nolint:funlen,gocognit,gocyclo
func main() {
	var err error
	if err = initApp(); err != nil {
		logrus.WithError(err).Fatal("initializing app")
	}

	if cfg.VersionAndExit {
		fmt.Printf("vault2env %s\n", version) //nolint:forbidigo
		os.Exit(0)
	}

	client, err := api.NewClient(&api.Config{
		Address: cfg.VaultAddress,
	})
	if err != nil {
		logrus.WithError(err).Fatal("creating vault client")
	}

	switch {
	case cfg.TokenAuth.Token != "":
		client.SetToken(cfg.TokenAuth.Token)

	case cfg.AppRoleAuth.RoleID != "":
		data := map[string]interface{}{
			"role_id": cfg.AppRoleAuth.RoleID,
		}
		if cfg.AppRoleAuth.SecretID != "" {
			data["secret_id"] = cfg.AppRoleAuth.SecretID
		}
		loginSecret, lserr := client.Logical().Write("auth/approle/login", data)
		if lserr != nil || loginSecret.Auth == nil {
			logrus.WithError(lserr).Fatal("fetching authentication token")
		}

		client.SetToken(loginSecret.Auth.ClientToken)
		defer func() {
			if err := client.Auth().Token().RevokeSelf(client.Token()); err != nil {
				logrus.WithError(err).Error("revoking approle-token")
			}
		}()

	default:
		logrus.Fatalf(strings.Join([]string{
			"[ERR] Did not find any authentication method. Try one of these:",
			"- Specify `--vault-token` for token based authentication",
			"- Specify `--vault-role-id` and optionally `--vault-secret-id` for AppRole authentication",
		}, "\n"))
	}

	envData := map[string]string{}

	for _, setName := range cfg.TransformSet {
		if setName == "" {
			continue
		}

		if set, ok := transformSets[setName]; ok {
			cfg.Transform = append(cfg.Transform, set...)
		} else {
			logrus.Warnf("transform set %q was not found, ignoring", setName)
		}
	}
	transformMap := env.ListToMap(cfg.Transform)

	for _, vaultKey := range cfg.VaultKeys {
		data, err := client.Logical().Read(vaultKey)
		if err != nil {
			logrus.WithError(err).Errorf("fetching data for key %q", vaultKey)
			continue
		}

		if data == nil {
			logrus.Errorf("vault key %q does not exist", vaultKey)
			continue
		}

		if data.Data == nil {
			logrus.Errorf("vault key %q did not contain data", vaultKey)
			continue
		}

		for k, v := range data.Data {
			key := k
			if newKey, ok := transformMap[key]; ok {
				key = newKey
			}

			switch vI := v.(type) {
			case string:
				envData[key] = vI
			case json.Number:
				envData[key] = string(vI)
			default:
				logrus.Errorf("vault key %q.%q contained unexpected data type %T", vaultKey, k, v)
				continue
			}
		}
	}

	if len(envData) == 0 {
		logrus.Fatalf("no environment data could be extracted")
	}

	if cfg.Export {
		for k, v := range envData {
			fmt.Printf("export %s=%q\n", k, v) //nolint:forbidigo
		}
		return
	}

	obfuscate := prepareObfuscator(envData)

	emap := env.ListToMap(os.Environ())
	for k, v := range emap {
		if _, ok := envData[k]; !ok {
			envData[k] = v
		}
	}

	cmd := exec.Command(rconfig.Args()[1], rconfig.Args()[2:]...) //#nosec:G204 // Intended to call user-defined process
	cmd.Stdin = os.Stdin
	cmd.Env = env.MapToList(envData)

	stderr, err := cmd.StderrPipe()
	if err != nil {
		logrus.WithError(err).Fatal("getting stderr pipe")
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		logrus.WithError(err).Fatal("getting stdout pipe")
	}

	if err := cmd.Start(); err != nil {
		logrus.WithError(err).Fatal("starting command")
	}

	wg := new(sync.WaitGroup)
	wg.Add(2) //nolint:gomnd

	go func() {
		defer wg.Done()
		if err := obfuscationTransport(stdout, os.Stdout, obfuscate); err != nil {
			logrus.WithError(err).Error("obfuscating stdout")
		}
	}()

	go func() {
		defer wg.Done()
		if err := obfuscationTransport(stderr, os.Stderr, obfuscate); err != nil {
			logrus.WithError(err).Error("obfuscating stderr")
		}
	}()

	wg.Wait()

	if err := cmd.Wait(); err != nil {
		logrus.WithError(err).Fatal("error during command execution")
	}
}
