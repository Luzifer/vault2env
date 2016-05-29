package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"

	"github.com/Luzifer/go_helpers/env"
	"github.com/Luzifer/rconfig"
	"github.com/hashicorp/vault/api"
)

var (
	cfg = struct {
		VaultAddress   string `flag:"vault-addr" env:"VAULT_ADDR" default:"https://127.0.0.1:8200" description:"Vault API address"`
		VaultAppID     string `flag:"vault-app-id" env:"VAULT_APP_ID" default:"" description:"The app-id to use for authentication"`
		VaultUserID    string `flag:"vault-user-id" env:"VAULT_USER_ID" default:"" description:"The user-id to use for authentication"`
		Export         bool   `flag:"export,e" default:"false" description:"Show export statements instead of running the command specified"`
		VersionAndExit bool   `flag:"version" default:"false" description:"Print program version and exit"`
	}{}
	version = "dev"
)

func init() {
	rconfig.Parse(&cfg)

	if cfg.VersionAndExit {
		fmt.Printf("vault2env %s\n", version)
		os.Exit(0)
	}

	if cfg.VaultAppID == "" || cfg.VaultUserID == "" {
		log.Fatalf("[ERR] You need to set vault-app-id and vault-user-id")
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

	loginSecret, err := client.Logical().Write("auth/app-id/login/"+cfg.VaultAppID, map[string]interface{}{
		"user_id": cfg.VaultUserID,
	})
	if err != nil || loginSecret.Auth == nil {
		log.Fatalf("Unable to fetch authentication token: %s", err)
	}

	client.SetToken(loginSecret.Auth.ClientToken)
	defer client.Auth().Token().RevokeSelf(client.Token())

	data, err := client.Logical().Read(rconfig.Args()[1])
	if err != nil {
		log.Fatalf("Unable to fetch data: %s", err)
	}

	if cfg.Export {
		for k, v := range data.Data {
			fmt.Printf("export %s=\"%s\"\n", k, v)
		}
		return
	}

	emap := env.ListToMap(os.Environ())
	for k, v := range data.Data {
		emap[k] = v.(string)
	}

	cmd := exec.Command(rconfig.Args()[2], rconfig.Args()[2:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Env = env.MapToList(emap)
	if err := cmd.Run(); err != nil {
		log.Fatal("Command exitted unclean (code != 0)")
	}
}
