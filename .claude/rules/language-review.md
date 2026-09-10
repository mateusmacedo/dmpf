# Revisão de idioma ao final da entrega

A revisão de prosa em PT-BR (`pt-reviewer`) e em inglês (`en-reviewer`) acontece
**uma vez, ao final da entrega**, sobre o conjunto de arquivos com prosa que a
tarefa alterou — não após cada escrita. Neste repositório, essa regra prevalece
sobre a instrução global de invocar o revisor logo após cada `Write`/`Edit`.

## Como aplicar

1. Durante a tarefa, escreva e edite sem disparar revisores.
2. Antes de declarar a entrega concluída, liste os arquivos tocados que contêm
   prosa (specs, ADRs, guias, READMEs, godocs, mensagens ao usuário) e invoque
   **um** `pt-reviewer` para o conjunto em PT-BR e **um** `en-reviewer` para o
   conjunto em inglês, em background, em paralelo.
3. Corrija o que for reportado e encerre. Arquivo novo: corrigir direto;
   arquivo preexistente: listar e confirmar antes de corrigir.

Entrega de arquivo único equivale a "final da entrega": revisa-se ao terminar
esse arquivo. Entregas longas (pipeline com várias etapas) revisam ao fim de
cada etapa que produz artefato para o usuário, não a cada arquivo.

[MOTIVO]: um revisor por arquivo multiplica latência e tokens sem ganho — a
prosa muda até o fim da tarefa e seria revisada de novo. Uma passada sobre o
conjunto final revisa cada texto uma vez, no estado em que será entregue.
