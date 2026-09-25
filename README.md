# Missale — Backend

API do app Missale: login com Apple e a orientação com o Jev (TypeSafe, via OpenRouter).
Molde do Hirefy, bem menor: Go numa Lambda + API Gateway HTTP, Cognito, Aurora DSQL.

## Estrutura (ports & adapters, igual ao Hirefy)

```
cmd/api                      Lambda (ou :8080 localmente) — só monta as peças
cmd/migrate                  aplica migrations/*.sql no DSQL como admin
pkg/config                   variáveis de ambiente
internal/core/domain         entidades e erros de domínio
internal/core/ports/inbound  AuthService, AccountService, DecisionService
internal/core/ports/outbound IdentityVerifier, AuthProvider, UserRepository,
                             UsageRepository, SubscriptionVerifier, DecisionEngine
internal/application/service regras: login, apagar conta, assinatura + limite + validação
internal/adapters/inbound/http            chi: router, handler/, middleware/
internal/adapters/outbound/auth/apple     token do Sign in with Apple
internal/adapters/outbound/auth/cognito   sessões (Cognito)
internal/adapters/outbound/persistence/dsql  users e decision_usage (Aurora DSQL)
internal/adapters/outbound/subscription/storekit  JWS do StoreKit 2
internal/adapters/outbound/decision/jev   Jev via OpenRouter
```

## Rotas

| Rota | Quem | O que faz |
|---|---|---|
| `POST /v1/auth/apple` `{identityToken}` | todos | valida o token da Apple, cria/acha o usuário no Cognito, devolve a sessão |
| `POST /v1/auth/refresh` `{refreshToken}` | todos | renova o access token |
| `DELETE /v1/account` | logado | apaga o usuário no banco e no Cognito |
| `POST /v1/decisions` `{state, questions}` | logado + assinante, ou dentro da cota grátis | repassa ao Jev; header `X-Subscription` = `jwsRepresentation` da transação StoreKit 2. Sem assinatura, cada conta tem `FREE_DECISIONS` (2) chamadas na vida: a orientação do onboarding |

- O texto do usuário (`state`) vai para o Jev e para nenhum outro lugar: não é logado nem gravado.
- O banco guarda só `users` (id, apple_sub), `decision_usage` (chamadas por dia, limite `DAILY_DECISION_LIMIT`) e `free_decisions` (chamadas grátis usadas).
- A assinatura é verificada pela cadeia de certificados da Apple (Apple Root CA G3), sem chamar a Apple.

## Deploy

```
./scripts/bootstrap-state.sh          # só uma vez: bucket do estado do Terraform
cp terraform/environments/dev/secrets.tfvars.example terraform/environments/dev/secrets.tfvars  # e preencher
make deploy ENV=dev                   # testes + build + terraform apply
make migrate ENV=dev                  # tabelas e papel missale_api no DSQL
```
