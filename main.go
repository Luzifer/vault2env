package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/Luzifer/go_helpers/env"
	"github.com/Luzifer/rconfig"
	"github.com/hashicorp/vault/api"
	"github.com/mitchellh/go-homedir"
)

var (
	cfg = struct {
		VaultAddress string `flag:"vault-addr" env:"VAULT_ADDR" default:"https://127.0.0.1:8200" description:"Vault API address"`
		AppRoleAuth  struct {
			RoleID   string `flag:"vault-role-id" env:"VAULT_ROLE_ID" default:"" description:"ID of the role to use"`
			SecretID string `flag:"vault-secret-id" env:"VAULT_SECRET_ID" default:"" description:"Corresponding secret ID to the role"`
		}
		TokenAuth struct {
			Token string `flag:"vault-token" env:"VAULT_TOKEN" vardefault:"vault-token" description:"Specify a token to use instead of app-id auth"`
		}
		Export         bool     `flag:"export,e" default:"false" description:"Show export statements instead of running the command specified"`
		Transform      []string `flag:"transform,t" default:"" description:"Translates keys to different names (oldkey=newkey)"`
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

	if cfg.Export {
		if len(rconfig.Args()) != 2 {
			log.Fatalf("[ERR] Usage: vault2env --export [secret path]")
		}
	} else {
		if len(rconfig.Args()) < 3 {
			log.Fatalf("[ERR] Usage: vault2env [secret path] [command]")
		}
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

	data, err := client.Logical().Read(rconfig.Args()[1])
	if err != nil {
		log.Fatalf("Unable to fetch data: %s", err)
	}

	transformMap := env.ListToMap(cfg.Transform)

	envData := map[string]string{}
	for k, v := range data.Data {
		key := k
		if newKey, ok := transformMap[key]; ok {
			key = newKey
		}
		envData[key] = v.(string)
	}

	if cfg.Export {
		for k, v := range envData {
			fmt.Printf("export %s=\"%s\"\n", k, v)
		}
		return
	}

	emap := env.ListToMap(os.Environ())
	for k, v := range emap {
		if _, ok := envData[k]; !ok {
			envData[k] = v
		}
	}

	cmd := exec.Command(rconfig.Args()[2], rconfig.Args()[3:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Env = env.MapToList(envData)
	if err := cmd.Run(); err != nil {
		log.Fatal("Command exitted unclean (code != 0)")
	}
}
