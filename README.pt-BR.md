# profadvisor

**[Read this in English / Leia em inglês](README.md)**

Mede onde um pacote Go gasta tempo ou memória e decide se uma mudança realmente
o melhorou. Ele não tem alvo próprio e não sabe nada sobre o código para o qual
é apontado: você informa um diretório e um padrão de pacote, e tudo o que ele
reporta vem do profile e da saída de benchmark daquela execução.

**Roda totalmente offline.** Sem chave de API, sem rede, sem fornecedor. Um
modelo de linguagem é opcional e externo: `prompt` monta a pergunta, você escolhe
quem responde, e `apply` recebe o patch que volta.

## Escopo

Lê profiles de CPU, de alocação, de contenção de bloqueio (block) e de contenção
de mutex sobre `go test -bench`. Profiles de trace não são cobertos, e ele não
captura latência de I/O, rede ou banco de dados — um alvo cujo custo está ali é
perfilado como se estivesse ocioso.

Todo comando reporta. Um comando julga: `verify` compara duas saídas de
benchmark e é o único lugar onde se chega a um veredito.

## Instalação

```
go build -o profadvisor .
```

Requer um toolchain Go no PATH — os benchmarks do alvo precisam compilar. Nada
além disso.

## Uso

Aponte para um repositório com `--dir` e para um pacote dentro dele com `--pkg`.

```
./profadvisor capture --dir /path/to/your/repo --pkg ./internal/parser/ --count 10
```

### Se o pacote ainda não tem benchmarks

Tudo aqui mede `go test -bench`, então um pacote sem benchmarks não tem nada a
perfilar. `benchgen` os escreve para você a partir de um diretório de arquivos de
corpus de fuzz do Go — um valor por parâmetro, na ordem dos parâmetros da função:

```sh
./profadvisor benchgen --dir /path/to/your/repo --pkg ./internal/parser \
  --func parse --corpus /path/to/seeds --out /path/to/artifacts --write
```

`--write` instala o arquivo de teste no pacote; `--out` guarda uma cópia que fica
fora do build. Você recebe um benchmark por seed e um alvo de fuzz que os
reexecuta. Depois, siga normalmente pelos passos abaixo.

O alvo de fuzz gerado verifica panics e nada mais: ele continua passando quando a
função é editada para devolver uma resposta errada. O formato do corpus, os tipos
de parâmetro aceitos e os passos de replay estão no [AGENTS.md](AGENTS.md).

### Encontrando o custo

`capture` roda o benchmark uma vez, guardando o profile e o `bench.txt` da mesma
execução. `extract` ordena o que encontrou:

```
./profadvisor extract profadvisor-out/<timestamp>/cpu.prof --format text
```

Frames do runtime e da biblioteca padrão são filtrados do ranking, e para onde
esse custo foi é reportado separadamente em `excluded`. Essa segunda lista muitas
vezes é o diagnóstico de verdade: uma função quente mais uma parede de
`runtime.concatstring2` diz que a correção é sobre alocação, não sobre o laço.

### Escolhendo o que otimizar

Uma flag seleciona o objetivo, e ela chega a todas as etapas — o que é perfilado
e qual métrica decide o veredito.

```
# tempo de CPU — o padrão, otimiza ns/op
./profadvisor capture --dir /path/to/your/repo --pkg ./internal/parser/

# alocação — otimiza B/op, ou allocs/op com --unit
./profadvisor capture --dir /path/to/your/repo --pkg ./internal/parser/ --profile memory

# contenção de lock ou de channel
./profadvisor capture --dir /path/to/your/repo --pkg ./internal/sync/ --profile mutex
./profadvisor capture --dir /path/to/your/repo --pkg ./internal/sync/ --profile block
```

Uma execução de memória leva `ns/op` como guarda: ele pode transformar o
resultado consolidado em `REGRESSED`, mas nunca em `IMPROVED`. Execuções de
contenção também são julgadas por `ns/op`. Registrar cada evento de contenção
adiciona overhead, então os números absolutos de uma captura de contenção não são
comparáveis aos de uma execução limpa; baseline e depois são capturados com
flags idênticas.

### Consultando um modelo — OPCIONAL

**Pule esta parte se você já sabe o que mudar.** Nada abaixo depende dela, e o
veredito nunca depende.

`prompt` transforma um documento de extract em um pedido que um modelo consegue
responder: o objetivo, os hotspots e o código-fonte deles.

```
./profadvisor prompt extract.json --format text
```

Ele não envia nada — só imprime texto. Cole em um chat, ou envie para a API que
você usar, e guarde o diff que voltar.

### Aplicando e julgando

Um diff — de um modelo, de um colega, seu — vai para uma branch própria:

```
./profadvisor apply patch.diff --dir /path/to/your/repo
```

Ele recusa uma árvore suja e, se o patch falhar depois que a branch existir, apaga
a branch e devolve você ao ponto de partida. Depois, capture de novo e compare:

```
./profadvisor verify --baseline <baseline bench.txt> --after <after bench.txt> --format text
```

Isso imprime `IMPROVED`, `NO CHANGE` ou `REGRESSED` por benchmark e métrica, com
o delta, um p-valor e o número de amostras. Os três vereditos saem com código 0.

### Lendo a saída

Todo comando escreve JSON no stdout e diagnósticos no stderr. `--format text`
renderiza o mesmo documento para uma pessoa.

## Comandos

| Comando | O que faz |
|---|---|
| `capture` | Roda o benchmark uma vez; guarda o profile e o `bench.txt`. |
| `extract` | Ordena as funções quentes, filtra o ruído do runtime, anexa o código-fonte. |
| `prompt` | Transforma um extract em pedido para um modelo. Não envia nada. |
| `apply` | Aplica um diff unificado em uma branch. Recusa árvore suja. |
| `verify` | O único comando que chega a um veredito, a partir de dois arquivos `bench.txt`. |
| `escape` | Reporta a análise de escape do compilador. Não precisa de benchmark. |
| `benchgen` | Gera testes de fuzz e benchmarks offline a partir de um corpus congelado. |

O [AGENTS.md](AGENTS.md) é a referência completa: cada flag, o contrato de I/O,
as versões de schema e o que cada veredito significa.

## Análise de escape

```
./profadvisor escape --dir /path/to/your/repo
```

Uma pergunta mais estreita, que não precisa de benchmark nem de profile: o que o
compilador concluiu sobre quais valores são alocados no heap. As conclusões são
do compilador, reportadas como JSON normalizado. Não há severidade, ranking nem
sugestão na saída: uma alocação no heap em um caminho que roda uma vez não custa
nada mensurável, e só um benchmark pode dizer se alguma delas importa.

## Dois fluxos de trabalho

Os dois deixam o modelo no fim, depois que a medição está feita. Nada do que ele
diz volta para a forma como os números foram produzidos.

### 1. Gerar uma mudança

O pipeline completo. Medir, achar o caminho quente, pedir um patch, aplicá-lo,
medir de novo e deixar a estatística decidir.

```sh
# só se o pacote ainda não tiver benchmarks
profadvisor benchgen --dir ~/svc --pkg ./internal/parser \
  --func parse --corpus ~/seeds --out ~/artifacts --write

# medir o estado atual
profadvisor capture --dir ~/svc --pkg ./internal/parser/ --count 10
#   -> profadvisor-out/<t1>/{cpu.prof,bench.txt}

# ordenar as funções quentes e anexar o código-fonte
profadvisor extract profadvisor-out/<t1>/cpu.prof > extract.json

# ---- OPCIONAL: o único passo em que um modelo entra ----
profadvisor prompt extract.json --format text > ask.txt
#   cole ask.txt em um chat, ou faça POST no seu próprio endpoint de API;
#   salve o diff unificado que voltar como patch.diff
#   (pule isto e escreva o patch.diff você mesmo — o resto é idêntico)
# --------------------------------------------------------

profadvisor apply patch.diff --dir ~/svc
#   -> aplicado na branch profadvisor/suggestion-1

# medir de novo, na branch em que o apply deixou você
profadvisor capture --dir ~/svc --pkg ./internal/parser/ --count 10
#   -> profadvisor-out/<t2>/{cpu.prof,bench.txt}

profadvisor verify \
  --baseline profadvisor-out/<t1>/bench.txt \
  --after    profadvisor-out/<t2>/bench.txt --format text
```

Quem decide é o `verify`, não o modelo. Se a resposta for `NO CHANGE` ou
`REGRESSED`, apague a branch — esse resultado custou um ciclo e é o caso normal, não
uma falha. Use o mesmo `--profile`/`--unit` nas duas capturas; comparar uma
execução de memória com uma de CPU é comparar programas diferentes.

### 2. Comparar dois estados

Sem geração nenhuma. Você já tem duas versões — uma release e a anterior, uma
branch e a `main`, antes e depois do patch de outra pessoa — e quer saber o que
mudou.

```sh
git checkout main
profadvisor capture --dir ~/svc --pkg ./internal/parser/ --count 10   # -> <t1>

git checkout my-branch
profadvisor capture --dir ~/svc --pkg ./internal/parser/ --count 10   # -> <t2>

profadvisor verify \
  --baseline profadvisor-out/<t1>/bench.txt \
  --after    profadvisor-out/<t2>/bench.txt --format text
```

Essa já é a resposta em tabela: uma linha por benchmark e métrica, com o
baseline, o depois, o delta, um p-valor e um veredito por linha.

O modelo, se você quiser um aqui, vem depois dessa tabela e a lê — "qual destas
regressões importa", "isto é consistente com o diff" — com os números já
definidos. Para dar a ele o *porquê* junto com o *quê*, rode `extract` nos dois
profiles e entregue-os também:

```sh
profadvisor extract profadvisor-out/<t1>/cpu.prof > before.json
profadvisor extract profadvisor-out/<t2>/cpu.prof > after.json
profadvisor verify --baseline ... --after ... > verdict.json
#   entregue verdict.json, before.json e after.json para quem você quiser
```

O `verify` lê o `bench.txt` em vez dos profiles: os profiles dizem para onde o
custo foi, o `bench.txt` diz quanto custo houve, e só o segundo pode ser
comparado entre execuções.

## Testes

```
go test ./...
```

A suíte é autocontida: roda contra um módulo descartável e um repositório git
temporário que os próprios testes criam, então nada fora deste checkout é
necessário e não há acesso à rede.
