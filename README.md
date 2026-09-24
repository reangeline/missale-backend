# Missale — Backend

API do app Missale: login com Apple e a orientação com o Jev (TypeSafe, via OpenRouter).
Molde do Hirefy, bem menor: Go numa Lambda + API Gateway HTTP, Cognito, Aurora DSQL.

## Rotas

| Rota | Quem | O que faz |
|---|---|---|
| `POST /v1/auth/apple` `{identityToken}` | todos | valida o token da Apple, cria/acha o usuário no Cognito, devolve a sessão |
| `POST /v1/auth/refresh` `{refreshToken}` | todos | renova o access token |
| `DELETE /v1/account` | logado | apaga o usuário no banco e no Cognito |
| `POST /v1/decisions` `{state, questions}` | logado + assinante | repassa ao Jev; header `X-Subscription` = `jwsRepresentation` da transação StoreKit 2 |

- O texto do usuário (`state`) vai para o Jev e para nenhum outro lugar: não é logado nem gravado.
- O banco guarda só `users` (id, apple_sub) e `decision_usage` (chamadas por dia, limite `DAILY_DECISION_LIMIT`).
- A assinatura é verificada pela cadeia de certificados da Apple (Apple Root CA G3), sem chamar a Apple.

## Deploy

```
./scripts/bootstrap-state.sh          # só uma vez: bucket do estado do Terraform
cp terraform/environments/dev/secrets.tfvars.example terraform/environments/dev/secrets.tfvars  # e preencher
make deploy ENV=dev                   # testes + build + terraform apply
make migrate ENV=dev                  # tabelas e papel missale_api no DSQL
```
