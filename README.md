[![Go Report Card](https://goreportcard.com/badge/github.com/Luzifer/vault2env)](https://goreportcard.com/report/github.com/Luzifer/vault2env)
![](https://badges.fyi/github/license/Luzifer/vault2env)
![](https://badges.fyi/github/downloads/Luzifer/vault2env)
![](https://badges.fyi/github/latest-release/Luzifer/vault2env)

# Luzifer / vault2env

`vault2env` is a really small utility to transfer fields of a key in [Vault](https://www.vaultproject.io/) into the environment. It uses the [`app-role`](https://www.vaultproject.io/docs/auth/approle.html) or simple [token authentication](https://www.vaultproject.io/docs/auth/token.html) to identify itself with the Vault server, fetches all fields in the specified keys and returns export directives for bash / zsh. That way you can do `eval` stuff and pull those fields into your ENV. If you don't want to use export directives you also can pass commands to `vault2env` to be executed using those environment variables.

## Usage

In general this program can either output your ENV variables to use with `eval` or similar or it can run a program with populated environment.

```console
$ vault2env --key=<secret path> <command>
<program is started, you see its output>

$ vault2env --export --key=<secret path>
export ...
```

For further examples and "special cases" see the Wiki: [Usage Examples](https://github.com/Luzifer/vault2env/wiki/Usage-Examples)

### Using evironment variables  
```bash
# export VAULT_ADDR="https://127.0.0.1:8200"
# export VAULT_ROLE_ID="29c8febe-49f5-4620-a177-20dff0fda2da"
# export VAULT_SECRET_ID="54d24f66-6ecb-4dcc-bdb7-0241a955f1df"
# vault2env --export --key=secret/my/path/with/keys
export FIRST_KEY="firstvalue"
export SECOND_KEY="secondvalue"

# eval $(vault2env --export --key=secret/my/path/with/keys)
# echo "${FIRST_KEY}"
firstvalue
```

### Using CLI parameters  

The command does differ only with its parameters specified for the different authentication mechanisms:

- When using AppRole you need to specify `--vault-role-id` and optionally `--vault-secret-id` if you're using the `bind_secret_id` flag for your AppRole
- When using Token auth only specify `--vault-token`

```bash
# vault2env --vault-addr="..." --vault-app-id="..." --vault-user-id="..." --key=secret/my/path/with/keys
export FIRST_KEY="firstvalue"
export SECOND_KEY="secondvalue"
```

Though it's possible to use CLI parameters I strongly recommend to stick to the ENV variant as it's possible under certain conditions to read CLI parameters on a shared system using for example `ps aux`.

----

![project status](https://d2o84fseuhwkxk.cloudfront.net/vault2env.svg)
