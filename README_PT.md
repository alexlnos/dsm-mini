<img src="docs/icon.png" width="88" alt="">

# dsm-mini

[English](README.md) · [Русский](README_RU.md) · [Español](README_ES.md) · **Português** · [Deutsch](README_DE.md) · [Français](README_FR.md) · [Italiano](README_IT.md) · [Türkçe](README_TR.md) · [Українська](README_UK.md) · [Polski](README_PL.md)

[![Verificações](https://github.com/alexlnos/dsm-mini/actions/workflows/ci.yml/badge.svg)](https://github.com/alexlnos/dsm-mini/actions/workflows/ci.yml)
[![Licença MIT](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)

Um bot do Telegram com Mini App para cuidar de um NAS Synology doméstico:
downloads do Download Station e arquivos do File Station direto do mensageiro,
sem VPN e sem a interface web do DSM.

Jogue um link magnet no chat — o bot pergunta com botões onde colocar e põe na
fila. Abra o aplicativo e você vê o que está baixando, quanto falta, o que há nos
discos e como o NAS está se sentindo.

<p align="center">
  <img src="docs/screenshots/pt-home.webp" width="19%" alt="Visão geral do NAS">
  <img src="docs/screenshots/pt-downloads.webp" width="19%" alt="Downloads">
  <img src="docs/screenshots/pt-task.webp" width="19%" alt="Uma tarefa">
  <img src="docs/screenshots/pt-files.webp" width="19%" alt="Arquivos">
  <img src="docs/screenshots/pt-storage.webp" width="19%" alt="Armazenamento">
</p>

> Funcionando: o bot, o Mini App e as notificações. Exige DSM 7 ou mais novo —
> no DSM 6 o pacote não instala e ninguém o testou lá. Testado no DSM 7.2.2
> com Download Station 4.1.2 e File Station 1.4.4.

## O que ele faz

**Downloads**

- A lista de tarefas: progresso, velocidade, tempo restante, seeds e peers
- Pausar, retomar, excluir
- Adicionar por link magnet, por link direto e por arquivo `.torrent`
- Escolher a pasta de destino, com uma lista de acesso rápido configurável
- Escolher os arquivos dentro de um torrent e a prioridade deles
- Uma mensagem no chat quando uma tarefa termina ou falha

**Arquivos**

- Navegar pelas pastas, pré-visualizar, enviar para o NAS
- Renomear, copiar, mover, excluir

**Estado do NAS**

- Carga de CPU e memória, tempo ligado, rede
- Discos, pools e volumes: temperatura, espaço usado, saúde
- Máquinas virtuais e contêineres: iniciar e parar
- O registro de eventos do DSM

**Notificações e configurações**

- O que o próprio DSM anuncia — consultor de segurança, discos, atualizações — chega ao chat
- A escolha do que o bot pode enviar: nada, apenas downloads, tudo
- Uma tela própria no menu principal do DSM: ali o pacote é configurado depois da instalação e ali se muda depois qualquer configuração, sem reiniciar

**Idioma**

O aplicativo e o bot falam o idioma escolhido no Telegram: inglês, russo,
espanhol, português, alemão, francês, italiano, turco, ucraniano, polonês. Um
idioma desconhecido recebe inglês.

---

## Instalação

Meia hora, e a maior parte vai no certificado, não no serviço. É preciso um NAS
Synology com DSM 7 e o Download Station; nada mais precisa ser instalado antes.

> **Por que é preciso um endereço público.** O Telegram abre um Mini App só por
> `https://` com certificado de verdade: um `192.168.…` local ou um
> autoassinado não abrem. O bot em si funciona sem endereço, só que sem o
> botão.

**1. Criar o bot.** [@BotFather](https://t.me/BotFather) → `/newbot` → um nome
e um usuário terminando em `bot`. Ele responde com um token; guarde, é a senha
do seu bot.

**2. Descobrir o seu número.** [@userinfobot](https://t.me/userinfobot) →
Start. Ele responde com a linha `Id`.

**3. Criar um usuário do DSM para o serviço.** Painel de Controle → Usuário e
grupo → Criar. Acesso só ao Download Station e ao File Station, sem verificação
em duas etapas: um código único não tem como vir de um arquivo de configuração.
Não dê um administrador: o serviço consegue excluir arquivos.

**4. Conseguir endereço e certificado.** Pule se o NAS já tem um domínio com
certificado válido.

- Painel de Controle → Acesso externo → DDNS → Adicionar, provedor `Synology`:
  sai algo como `alex-nas.synology.me`.
- No roteador, encaminhe para o NAS as portas **80** e **443**. Sem a 80 o
  certificado não será emitido, sem a 443 o aplicativo não abre.
- Painel de Controle → Segurança → Certificado → Adicionar → da Let's Encrypt,
  para esse mesmo nome.

Confira pelo celular na internet móvel: `https://alex-nas.synology.me:5001`
deve abrir o DSM sem avisos.

**5. Instalar o pacote.** Central de Pacotes → Configurações → Origens de
pacotes → Adicionar, nome `dsm-mini` e o endereço da sua arquitetura:

- Intel e AMD, a maioria dos modelos: `https://alexlnos.github.io/dsm-mini/amd64.json`
- ARM, modelos de entrada: `https://alexlnos.github.io/dsm-mini/arm64.json`

Depois Configurações → Geral → Nível de confiança → **Qualquer editor**, e
instale **DSM mini — Telegram Mini App** na seção **Comunidade**. Na dúvida
sobre a arquitetura? Tente `amd64`: um pacote que não serve é simplesmente
recusado.

A instalação não pergunta nada. Ou instale o `.spk` das
[versões](https://github.com/alexlnos/dsm-mini/releases) na mão, pela Central de
Pacotes → Instalação manual.

**6. Configurar.** Abra o **DSM mini** pelo menu principal do DSM ou aperte
**Abrir** na Central de Pacotes. Preencha os cinco valores — tudo para eles foi
reunido nos passos acima, e a janela explica cada um — e aperte **Salvar e
iniciar**. A linha de estado no topo da janela mostra quando o DSM e o bot
responderam, e o que está errado se não responderam.

A regra de proxy reverso para o endereço a própria janela cria: digite o seu
nome em **Apontar um nome ao serviço**. Na mão é Painel de Controle → Portal de
login → Avançado → Proxy reverso → Criar. Origem: `HTTPS`, o seu nome, porta
`443`. Destino: `HTTP`, `localhost`, porta `58080`.

> Não faça proxy da porta **80** para esse nome: é por ela que o DSM renova o
> certificado, e interceptar quebra a renovação três meses depois.

**7. Conferir.** `https://seu-endereco/healthz` num navegador deve responder
`{"status":"ok"}`. Sem assinatura do Telegram nada é entregue.

**8. Abrir o aplicativo.** Ache o bot pelo usuário, aperte Start e ao lado do
campo de escrita aparece um botão **Downloads**. Mande a ele qualquer link
magnet: ele oferece pastas com botões.

## Se algo deu errado

| O que você vê | Qual é o problema | O que fazer |
|---|---|---|
| O bot fica mudo no `/start` | O pacote não está configurado, ou o token está errado | Abra o **DSM mini** no menu principal do DSM: a linha de estado no topo diz qual |
| «O acesso a este bot está fechado» | Seu ID não está na lista | Ponha o número do passo 2 nos IDs permitidos na janela **DSM mini** |
| Não há botão do aplicativo | O endereço público está vazio ou não é `https://` | A janela **DSM mini**, endereço público |
| O botão existe, o aplicativo não abre | O proxy reverso ou o certificado não funcionam | Abra `https://seu-endereco/healthz` num navegador |
| «Abra o aplicativo pelo bot» | O aplicativo foi aberto por link direto no navegador | É assim mesmo: abra pelo bot |
| «Acesso negado: o seu ID do Telegram…» | O serviço não reconheceu você | Nos IDs permitidos só dígitos, separados por vírgula |
| A janela diz que o DSM recusou o login | A senha, a 2FA ou as permissões do usuário | Passo 3: senha sem erro de digitação, 2FA desligada, aplicativos permitidos |
| A janela diz que o Download Station não está rodando | Não está instalado ou está negado ao usuário | Central de Pacotes e as permissões do passo 3 |
| O pacote para logo depois de subir | A porta dele está ocupada por outra coisa — o registro diz | Mude `LISTEN_ADDR` no `config.env` por SSH (veja abaixo) |

O primeiro lugar para olhar é a linha de estado no topo da janela **DSM mini**:
ela diz se o DSM e o Telegram responderam e, se não, por quê. Os detalhes ficam
no registro do pacote, `/var/packages/dsm-mini/var/dsm-mini.log`, que também
abre pela Central de Pacotes. O registro é em inglês; a janela, a interface e as
mensagens do bot são no seu idioma.

### Mudar as configurações

**Abra o DSM mini no menu principal do DSM** — todas as configurações estão ali.
A senha e o token são apenas de escrita: um campo vazio mantém o que havia.
Salvar vale de imediato: o serviço se reinicia sozinho com as novas
configurações, e o pacote não precisa ser reiniciado.

As configurações ficam num único arquivo do NAS,
`/var/packages/dsm-mini/var/config.env`, com permissões `600`. Dá para editá-lo
também por SSH (Painel de Controle → Terminal e SNMP → ligar o SSH) e depois
reiniciar o pacote:

```bash
sudo sed -i "s|^TELEGRAM_BOT_TOKEN=.*|TELEGRAM_BOT_TOKEN='seu-novo-token'|" /var/packages/dsm-mini/var/config.env
sudo synopkg restart dsm-mini
```

Uma atualização não mexe no arquivo, e é por isso que as configurações
sobrevivem.

## Atualizar

Se a origem de pacotes foi adicionada, a Central de Pacotes mostra a atualização
sozinha. Sem origem, baixe o `.spk` novo das
[versões](https://github.com/alexlnos/dsm-mini/releases) e instale por cima.

As configurações e o banco ficam nos dois casos: eles moram no diretório `var` do
pacote, e uma atualização não mexe nele.

## Onde os dados ficam

Tudo está em `/var/packages/dsm-mini/var/`:

- `config.env` — as configurações da janela DSM mini, permissões `600`;
- `dsm-mini.db` — um banco SQLite: as pastas fixadas, o idioma e os últimos
  estados conhecidos das tarefas, pelos quais o serviço sabe do que já avisou;
- `dsm-mini.log` — o registro.

Atualizar o pacote mantém os três. Desinstalar apaga.

## Segurança

O serviço está exposto à internet e consegue excluir arquivos no NAS, por isso:

- **Um usuário do DSM separado**, não um administrador (passo 3).
- **Sem verificação em duas etapas** nessa conta: um código único não tem como
  funcionar a partir de um arquivo de configuração.
- **Os IDs permitidos são uma lista de liberados.** Uma lista vazia fecha o
  acesso a todos, não abre.
- Cada pedido a `/api/` é verificado duas vezes: a assinatura do `initData` com
  uma chave derivada do token do bot, e o identificador contra a lista. A
  assinatura só prova que alguém abriu o bot — e abrir, qualquer um pode.
- Para fora vai uma frase genérica e os detalhes para o registro: uma mensagem
  dizendo o que exatamente não bateu na assinatura seria uma dica de como
  forjá-la.
- O token do bot e a senha do DSM moram só no `config.env`, no próprio NAS, com
  permissões `600`, e nunca chegam ao registro: numa recusa escreve-se o motivo,
  nunca o valor.
- O serviço escuta só em `127.0.0.1`, ou seja, a partir do próprio NAS. Tudo que
  vem de fora passa pelo proxy reverso do DSM, que também encerra o TLS.

## Desenvolvimento

```bash
cd web && npm install && npm run build   # o Mini App vai parar em internal/web/dist
cd .. && go build ./cmd/dsm-mini         # um binário com o aplicativo embutido
go test ./...
```

O frontend separado, com recarga automática:

```bash
cd web && npm run dev    # conversa com o backend em localhost:58080
```

Rodar o backend localmente lê o `config.env` em `STATE_DIR`, como o pacote faz,
e as variáveis de ambiente completam o que o arquivo não diz. Fica prático
mantê-las num arquivo:

```bash
cp .env.example .env     # preencha
set -a; . ./.env; set +a
go run ./cmd/dsm-mini
```

A interface compilada está no repositório (`internal/web/dist`) — é de lá que o
`go:embed` pega. Depois de mexer no frontend, recompile e faça commit do
resultado, senão as verificações não passam.

Os testes de integração rodam contra um NAS real e são pulados por padrão:

```bash
DSM_URL=https://192.168.1.10:5001 DSM_USER=... DSM_PASSWORD=... \
  DSM_INSECURE_TLS=true go test -tags=integration ./...
```

Os testes que mudam o estado do NAS precisam de permissão à parte:
`DSM_TEST_MUTATIONS=1`, e para as operações de arquivo também `DSM_TEST_FOLDER` —
a pasta dentro da qual é permitido criar arquivos temporários. Eles apagam tudo o
que criam.

Um idioma novo da interface é um arquivo de dicionário de cada lado:
`web/src/i18n/<código>.ts` e `internal/i18n/<código>.go`. Não dá para esquecer uma
linha: no frontend o tipo cuida disso, no servidor um teste.

O código, os comentários e o registro do projeto são em inglês; os outros idiomas
moram só nos dicionários. Os detalhes estão em [CLAUDE.md](CLAUDE.md).

## Particularidades da API da Synology

A Web API da Synology às vezes se comporta de forma diferente da documentação: a
mesma ação responde em três formatos diferentes, `limit = -1` derruba o File
Station, e o `_sid` no envio de um arquivo precisa ser passado de outro jeito que
no resto. Tudo o que foi descoberto num NAS real está reunido em
[docs/synology-api.md](docs/synology-api.md).

## Licença

[MIT](LICENSE)
