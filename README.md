[![Download on GoBuilder](http://badge.luzifer.io/v1/badge?title=Download%20on&text=GoBuilder)](https://gobuilder.me/github.com/Luzifer/vault2env)
[![License: Apache v2.0](https://badge.luzifer.io/v1/badge?color=5d79b5&title=license&text=Apache+v2.0)](http://www.apache.org/licenses/LICENSE-2.0)
[![Go Report Card](https://goreportcard.com/badge/github.com/Luzifer/vault2env)](https://goreportcard.com/report/github.com/Luzifer/vault2env)

# Luzifer / vault2env

`vault2env` is a really small utility to transfer fields of a key in [Vault](https://www.vaultproject.io/) into the environment. It uses the [`app-id` authentication mechanism](https://www.vaultproject.io/docs/auth/app-id.html) to identify itself with the Vault server, fetches all fields in the specified key and returns export directives for bash / zsh. That way you can do `eval` stuff and pull those fields into your ENV.

## Usage

You have two ways to set the four input values the tool needs.

### Using evironment variables  
```bash
# export VAULT_ADDR="https://127.0.0.1:8200"
# export VAULT_APP_ID="29c8febe-49f5-4620-a177-20dff0fda2da"
# export VAULT_USER_ID="54d24f66-6ecb-4dcc-bdb7-0241a955f1df"
# vault2env secret/my/path/with/keys
export FIRST_KEY="firstvalue"
export SECOND_KEY="secondvalue"
# eval $(vault2env secret/my/path/with/keys)
# echo "${FIRST_KEY}"
firstvalue
```

### Using CLI parameters  
```bash
# vault2env --vault-addr="..." --vault-app-id="..." --vault-user-id="..." secret/my/path/with/keys
export FIRST_KEY="firstvalue"
export SECOND_KEY="secondvalue"
```

Though it's possible to use CLI parameters I strongly recommend to stick to the ENV variant as it's possible under certain conditions to read CLI parameters on a shared system using for example `ps aux`.
