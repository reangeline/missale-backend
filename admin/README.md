# Missale · Conteúdo (painel)

Página onde se edita e publica o conteúdo do app (Next.js 16, shadcn `base-nova`,
como o web-app do Hirefy). Fala só com as rotas `/v1/admin` da API.

```
cp .env.example .env.local   # NEXT_PUBLIC_API_URL da API (dev ou prod)
npm install && npm run dev   # http://localhost:3000
```

- Login com e-mail e senha de um admin (`../scripts/create-admin.sh`). No primeiro
  acesso a página pede para trocar a senha provisória que chegou por e-mail.
- Editar salva **rascunho** no banco. **Publicar** gera os arquivos que o app baixa.
- Os formulários vêm dos campos que a API declara (`GET /v1/admin/collections`):
  uma coleção nova no backend aparece aqui sem mudar a página.
