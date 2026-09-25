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

## Painel de conteúdo (`/v1/admin`)

Login com e-mail e senha (cliente `admin` do Cognito, grupo `admin`). Criar um admin:
`./scripts/create-admin.sh dev email@exemplo.com` — o Cognito manda a senha provisória por e-mail.

| Rota | O que faz |
|---|---|
| `POST /v1/admin/auth/signin` `{email,password}` | sessão, ou `newPasswordNeeded` + `session` no primeiro acesso |
| `POST /v1/admin/auth/new-password` `{email,session,newPassword}` | troca a senha provisória |
| `POST /v1/admin/auth/refresh` `{refreshToken}` | renova a sessão |
| `GET /v1/admin/collections` | coleções e campos (a página monta os formulários daqui) |
| `GET /v1/admin/content/{coleção}/{idioma}` | itens em ordem |
| `PUT /v1/admin/content/{coleção}/{idioma}/{id}` `{data, position?}` | cria ou edita (rascunho) |
| `DELETE /v1/admin/content/{coleção}/{idioma}/{id}` | apaga |
| `POST /v1/admin/publish` | gera `v{N}/{coleção}/{idioma}.json` e o `manifest.json` no S3/CloudFront |
| `GET /v1/admin/releases` | publicações |

O app baixa `manifest.json` do CloudFront, compara o hash de cada arquivo e baixa só o que mudou.
Idioma sem itens no painel não é publicado: o app continua com a lista embutida.

## Fluxo (gitflow, como no Hirefy)

```
feature/fix → PR para develop → CI (vet, testes, build) + terraform plan do dev → merge
            → deploy automático no dev (deploy-dev.yml: apply, migrações, /health)
            → validar no dev (o TestFlight usa o dev) → PR develop → main → CI + plan do prod → merge
            → deploy-prod.yml espera a aprovação no ambiente "production" (Actions › Review deployments)
```

- O GitHub entra na AWS pelo papel `missale-github-deploy` (OIDC, sem chave fixa),
  criado por `terraform/bootstrap` e restrito a este repositório.
- Segredo do repositório: `OPENROUTER_API_KEY`. Variável: `AWS_DEPLOY_ROLE_ARN`.
- `main` e `develop` são protegidas: só entram por PR, com o check `Test` verde.
- O repositório é público; nenhum segredo fica no código. Planos do Terraform
  (`*.tfplan`) guardam as variáveis e ficam fora do git.

## Deploy à mão (se preciso)

```
./scripts/bootstrap-state.sh          # só uma vez: bucket do estado do Terraform
cp terraform/environments/dev/secrets.tfvars.example terraform/environments/dev/secrets.tfvars  # e preencher
make deploy ENV=dev                   # testes + build + terraform apply
make migrate ENV=dev                  # tabelas e papel missale_api no DSQL (pode repetir)
```

Planos do Terraform (`*.tfplan`) guardam as variáveis, inclusive a chave do
OpenRouter: ficam fora do git.
