# Holdotfiles

Backup e instalação de dotfiles em ZIP usando Cloudflare R2, com interface de terminal e CLI.

**Nome do projeto: Holdotfiles. Comando: `hdt`.** Sem argumentos, abre a TUI.

## Instalação rápida

Requisitos: Linux, curl, tar e Go 1.24.1 ou superior. O instalador baixa o
código e as dependências Go, então precisa de acesso à internet. Não é necessário
instalar Git nem usar sudo.

```bash
curl -fsSL https://raw.githubusercontent.com/lunebakami/holdotfiles-go/main/install.sh | sh
```

O instalador compila a versão publicada no GitHub e instala em `~/.local/bin/hdt`. Ele cria
modelos de configuração sem sobrescrever arquivos existentes. Para atualizar,
execute o mesmo comando novamente. Também é possível clonar o repositório e rodar
`sh install.sh` para compilar alterações locais.

Se `~/.local/bin` ainda não estiver no PATH, adicione ao seu `~/.zshrc`:

```zsh
export PATH="$HOME/.local/bin:$PATH"
```

Alternativas: `make install` usa o mesmo instalador;
`sh install.sh --prefix /caminho/absoluto` instala em `/caminho/absoluto/bin/hdt`.
Quem usa Go pode executar `go install ./cmd/hdt` no checkout, com instalação
em GOBIN/GOPATH; nesse caso os modelos de configuração não são criados.

## O que ele faz

- lê arquivos e diretórios de `~/.hdtconfig`;
- percorre diretórios recursivamente;
- compacta os caminhos em um ZIP cuja raiz representa o diretório pessoal;
- envia um novo objeto `<computador>/backup-AAAA-MM-DDTHH-MM-SS.nanosZ.zip` para o R2;
- calcula SHA-256 do ZIP para verificar sua integridade na restauração;
- mantém máquinas separadas por um prefixo (por padrão, o hostname);
- mantém cada versão como um ZIP separado com data/hora UTC no nome;
- permite cancelar uma sincronização em andamento com `x`.

Restauração disponível na TUI e CLI: mostra uma prévia e instala com cópia dos arquivos substituídos.
Na TUI, `r` carrega as versões, as setas selecionam uma e Enter mostra a prévia.
Na CLI, `hdt --list` mostra cada chave; `hdt --restore 'hostname/backup-DATA.zip'`
seleciona uma versão específica. `hdt --restore hostname` usa a mais recente.
Cada envio cria uma versão; versões antigas não são removidas automaticamente.
O formato legado `hostname/backup.zip` também continua visível e restaurável.

## Configuração

1. Crie um bucket Standard no painel do Cloudflare R2.
2. Crie um API Token limitado ao bucket, com permissão Object Read & Write.
3. Preencha `~/.config/holdotfiles/.env` (criado pelo instalador) com as credenciais. Se usa XDG_CONFIG_HOME, o arquivo fica em `$XDG_CONFIG_HOME/holdotfiles/.env`.
4. Crie `~/.hdtconfig`, com um caminho por linha:

```text
# Arquivos individuais
~/.zshrc
~/.gitconfig

# Diretórios são percorridos recursivamente
~/.config/ghostty
```

Linhas vazias, comentários iniciados por `#` e caminhos repetidos são ignorados.

Exemplo de credenciais:

```dotenv
R2_ACCOUNT_ID=seu_account_id
R2_ACCESS_KEY_ID=sua_access_key
R2_SECRET_ACCESS_KEY=sua_secret_key
R2_BUCKET=holdotfiles
# Opcional; o padrão é o nome do computador:
R2_PREFIX=meu-computador
```

Um endpoint explícito pode ser definido em `R2_ENDPOINT`; uma lista alternativa
de origens pode ser escolhida com `HOLDOTFILES_CONFIG`.
Precedência: variáveis exportadas > `.env` do diretório atual > arquivo global.
Assim, `hdt` pode ser usado de qualquer pasta. No desenvolvimento, o `.env`
local continua funcionando. O instalador não copia credenciais do projeto.

Para migrar uma instalação anterior, transfira os valores do `.env` local
para o arquivo global. Não inclua esse arquivo de credenciais no backup.
O ZIP não possui criptografia própria; o bucket deve permanecer privado.

## Execução

```bash
make test
make build
./bin/hdt
```

Na interface, use `Tab` para trocar de tela, `s` para sincronizar, `x` para cancelar e `q` para sair.

Use `r` para abrir/atualizar a tela **Restaurar backup**. Ela mostra computador,
data do último envio no horário local e tamanho do ZIP, com os mais recentes
primeiro. Selecione com `↑/↓` e pressione `Enter` para baixar e verificar a
prévia. Use `i` para confirmar a instalação ou `Esc` para voltar. Na prévia,
as setas rolam os arquivos. A pasta de recuperação aparece ao concluir.
É possível abrir a TUI para restaurar mesmo sem `~/.hdtconfig`; nesse caso,
o envio fica indisponível até configurar os caminhos e reabrir o programa.
O argumento `--dest` também define o destino da restauração na TUI.

A data vem do campo LastModified do R2 e está disponível para ZIPs já enviados.
Cada envio cria uma versão, mesmo sem alterações. Os nomes usam data/hora UTC;
a interface mostra o horário local. Nenhuma versão é removida automaticamente.
O antigo `<computador>/backup.zip` continua disponível para restauração.
`hdt --list` mostra as chaves completas; use `hdt --restore 'computador/backup-DATA.zip'`
para uma versão específica, ou `hdt --restore computador` para a mais recente.
O histórico aumenta o consumo de armazenamento do R2.

O projeto se chama **Holdotfiles**; o comando é **hdt**. Para instalar:

```bash
make install
hdt
```

O diretório `~/.local/bin` precisa estar no `PATH`. Sem argumentos, `hdt` abre a TUI.

## Formato

Para as origens `~/.config/nvim` e `~/kitty.conf`, o ZIP contém:

```text
backup.zip
├── .config/
│   └── nvim/
│       └── init.lua
└── kitty.conf
```

O prefixo é o hostname ou `R2_PREFIX`. A raiz do ZIP sempre representa `~`,
sem incluir o nome do usuário de origem. Diretórios vazios são preservados.
Links simbólicos para arquivos são seguidos: o ZIP guarda o conteúdo do alvo como
arquivo comum no caminho original do link. Isso inclui alvos fora de `~`; o
caminho configurado do link deve estar dentro de `~`. A restauração não recria
o link nem acompanha alterações futuras no alvo.
Origens fora de `~`, links quebrados, links para diretórios e arquivos especiais
são rejeitados. A validação de links no destino da instalação permanece ativa.
Se uma origem falhar, nenhum ZIP parcial será enviado.

## Backup e restauração

```bash
# Enviar sem abrir a TUI
./bin/hdt --backup

# Descobrir os nomes de computadores
./bin/hdt --list

# Baixar e mostrar a prévia, sem instalar
./bin/hdt --restore nome-do-computador

# Instalar no diretório pessoal atual
./bin/hdt --restore nome-do-computador --apply

# Instalar em outro diretório
./bin/hdt --restore nome-do-computador --dest /tmp/dotfiles-demo --apply
```

Listagem e restauração não exigem `~/.hdtconfig` na máquina de destino.
As credenciais R2 continuam necessárias. A restauração verifica SHA-256,
valida os caminhos e extrai todo o ZIP em staging antes de iniciar a instalação.
Entradas que escapam do destino e destinos com links simbólicos são rejeitados.
Os arquivos anteriores são preservados em `.holdotfiles-recovery-*` dentro do
destino, com a mesma hierarquia. O programa imprime esse caminho inclusive se a
instalação falhar parcialmente. Use `--recover` para repor os arquivos dessa cópia.
Arquivos novos não possuem cópia anterior.
Não há rollback automático do conjunto nem remoção de arquivos locais extras.
Permissões dos arquivos são preservadas; diretórios novos usam permissão privada.

O protótipo limita o conteúdo descompactado e o download a 1 GiB.
Backups antigos de objetos individuais permanecem no bucket, mas não aparecem
em `--list`; execute um novo backup para gerar o ZIP.

## Recuperação local

```bash
# Listar cópias disponíveis no diretório pessoal
./bin/hdt --recoveries

# Conferir o que será recuperado
./bin/hdt --recover .holdotfiles-recovery-123

# Repor os arquivos anteriores
./bin/hdt --recover .holdotfiles-recovery-123 --apply
```

Troque o nome pelo exibido em `--recoveries`. Para uma instalação feita com
`--dest`, informe o mesmo destino tanto na listagem quanto na recuperação:

```bash
./bin/hdt --recoveries --dest /tmp/dotfiles-demo
./bin/hdt --recover .holdotfiles-recovery-123 --dest /tmp/dotfiles-demo --apply
```

Esse fluxo funciona offline, sem `.env` nem `~/.hdtconfig`. A cópia selecionada
permanece intacta. Os arquivos atuais substituídos são guardados em uma nova
pasta de recuperação, permitindo desfazer a recuperação pelo mesmo comando.
Apenas os arquivos presentes na cópia são repostos: arquivos novos ou extras
permanecem no destino. Pastas vazias de recuperação retornam uma mensagem sem
alterar arquivos.

## Testes

```bash
go test -race ./...
go vet ./...
# Opt-in: usa o .env, envia apenas dados sintéticos e remove o objeto remoto.
HOLDOTFILES_LIVE_TEST=1 go test ./internal/storage -run '^TestR2Live$' -v -count=1
```
