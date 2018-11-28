package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"sync"

	log "github.com/Sirupsen/logrus"
	"github.com/hashicorp/vault/api"
	"github.com/mitchellh/go-homedir"

	"github.com/Luzifer/go_helpers/env"
	"github.com/Luzifer/rconfig"
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

	data, err := ioutil.ReadFile(vf)
	if err != nil {
		return ""
	}

	return string(data)
}

func init() {
	rconfig.SetVariableDefaults(map[string]string{
		"vault-token": vaultTokenFromDisk(),
	})
	rconfig.Parse(&cfg)

	if cfg.VersionAndExit {
		fmt.Printf("vault2env %s\n", version)
		os.Exit(0)
	}

	if logLevel, err := log.ParseLevel(cfg.LogLevel); err == nil {
		log.SetLevel(logLevel)
	} else {
		log.Fatalf("Unable to parse log level: %s", err)
	}

	if len(cfg.VaultKeys) == 0 || (len(cfg.VaultKeys) == 1 && cfg.VaultKeys[0] == "") {
		log.Fatalf("[ERR] You need to specify at least one --key to read")
	}

	if !cfg.Export && len(rconfig.Args()) == 1 {
		log.Fatalf("[ERR] Usage: vault2env [command]")
	}
}

func main() {
	client, err := api.NewClient(&api.Config{
		Address: cfg.VaultAddress,
	})
	if err != nil {
		log.Fatalf("Unable to create client: %s", err)
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
			log.Fatalf("Unable to fetch authentication token: %s", lserr)
		}

		client.SetToken(loginSecret.Auth.ClientToken)
		defer client.Auth().Token().RevokeSelf(client.Token())

	default:
		log.Fatalf(strings.Join([]string{
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
			log.Warnf("Transform set %q was not found, ignoring", setName)
		}
	}
	transformMap := env.ListToMap(cfg.Transform)

	for _, vaultKey := range cfg.VaultKeys {
		data, err := client.Logical().Read(vaultKey)
		if err != nil {
			log.Errorf("Unable to fetch data for key %q: %s", vaultKey, err)
			continue
		}

		if data == nil {
			log.Errorf("Vault key %q does not exist", vaultKey)
			continue
		}

		if data.Data == nil {
			log.Errorf("Vault key %q did not contain data", vaultKey)
			continue
		}

		for k, v := range data.Data {
			key := k
			if newKey, ok := transformMap[key]; ok {
				key = newKey
			}
			envData[key] = v.(string)
		}
	}

	if len(envData) == 0 {
		log.Fatalf("No environment data could be extracted")
	}

	if cfg.Export {
		for k, v := range envData {
			fmt.Printf("export %s=%q\n", k, v)
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

	cmd := exec.Command(rconfig.Args()[1], rconfig.Args()[2:]...)
	cmd.Stdin = os.Stdin
	cmd.Env = env.MapToList(envData)

	stderr, err := cmd.StderrPipe()
	if err != nil {
		log.WithError(err).Fatal("Unable to get stderr pipe")
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.WithError(err).Fatal("Unable to get stdout pipe")
	}

	if err := cmd.Start(); err != nil {
		log.WithError(err).Fatal("Unable to start command")
	}

	wg := new(sync.WaitGroup)
	wg.Add(2)

	go func() {
		defer wg.Done()
		obfuscationTransport(stdout, os.Stdout, obfuscate)
	}()

	go func() {
		defer wg.Done()
		obfuscationTransport(stderr, os.Stderr, obfuscate)
	}()

	wg.Wait()

	if err := cmd.Wait(); err != nil {
		log.Fatal("Command exitted unclean (code != 0)")
	}
}
