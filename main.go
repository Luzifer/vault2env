package main

import (
	"fmt"
	"log"

	"github.com/Luzifer/rconfig"
	"github.com/hashicorp/vault/api"
)

var (
	cfg = struct {
		VaultAddress string `flag:"vault-addr" env:"VAULT_ADDR" default:"https://127.0.0.1:8200" description:"Vault API address"`
		VaultAppID   string `flag:"vault-app-id" env:"VAULT_APP_ID" default:"" description:"The app-id to use for authentication"`
		VaultUserID  string `flag:"vault-user-id" env:"VAULT_USER_ID" default:"" description:"The user-id to use for authentication"`
	}{}
	version = "dev"
)

func init() {
	rconfig.Parse(&cfg)

	if cfg.VaultAppID == "" || cfg.VaultUserID == "" {
		log.Fatalf("[ERR] You need to set vault-app-id and vault-user-id")
	}

	if len(rconfig.Args()) != 2 {
		log.Fatalf("[ERR] Exactly one argument is supported: The path of the key containing env variables")
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

	data, err := client.Logical().Read(rconfig.Args()[1])
	if err != nil {
		log.Fatalf("Unable to fetch data: %s", err)
	}

	for k, v := range data.Data {
		fmt.Printf("export %s=\"%s\"\n", k, v)
	}

	if err := client.Auth().Token().RevokeSelf(client.Token()); err != nil {
		log.Printf("Unable to clean up: %s", err)
	}
}
