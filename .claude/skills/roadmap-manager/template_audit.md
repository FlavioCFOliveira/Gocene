---
skill_name: "[NOME_DA_SKILL]"
audit_date: "YYYY-MM-DD"
specialty: "[EX: SEGURANÇA / PERFORMANCE / BACKEND]"
summary:
  alta_prioridade: 0
  media_prioridade: 0
  baixa_prioridade: 0
status: "CONCLUÍDO"
---

# RELATÓRIO DE AUDITORIA TÉCNICA: [NOME_DA_SKILL]

## 1. RESUMO DA ESPECIALIDADE

Análise técnica objetiva do estado atual do projeto nesta especialidade. Identificação de riscos imediatos e
oportunidades de melhoria estrutural.

## 2. LISTA DE TAREFAS POR SEVERIDADE

As tarefas abaixo devem ser descritas como ordens de execução diretas para fluxos agentic.

| ID          | SEVERIDADE | TAREFA         | DESCRIÇÃO TÉCNICA ACIONÁVEL                |
|:------------|:-----------|:---------------|:-------------------------------------------|
| [SKILL]-001 | ALTA       | Nome da Tarefa | Instrução técnica detalhada para execução. |
| [SKILL]-002 | MÉDIA      | Nome da Tarefa | Instrução técnica detalhada para execução. |
| [SKILL]-003 | BAIXA      | Nome da Tarefa | Instrução técnica detalhada para execução. |

## 3. EVIDÊNCIAS TÉCNICAS E DIAGNÓSTICO DETALHADO

Esta secção fundamenta as tarefas listadas acima com provas extraídas do código.

### ID: [SKILL]-001 (ALTA)

- **Localização**: `src/api/auth.ts:45`
- **Problema**: Ausência de validação de tokens JWT no middleware de rota.
- **Impacto**: Possível bypass de autenticação em ambientes de produção.
- **Sugestão de Solução**: Implementar a biblioteca `jsonwebtoken` e verificar a assinatura no `request header`.

### ID: [SKILL]-002 (MÉDIA)

- **Localização**: `src/components/DataList.tsx:112`
- **Problema**: O loop de renderização não utiliza `memoization`, causando re-renders em cada alteração de estado
  global.
- **Impacto**: Degradação da performance da UI em listas com mais de 100 itens.
- **Sugestão de Solução**: Aplicar `useMemo` nos cálculos de filtragem e `React.memo` no componente de linha.

### ID: [SKILL]-003 (BAIXA)

- **Localização**: `README.md`
- **Problema**: As instruções de instalação estão desatualizadas em relação à versão atual do Node.js utilizada no
  projeto.
- **Impacto**: Dificuldade no onboarding de novos developers.
- **Sugestão de Solução**: Atualizar a secção "Prerequisitos" para Node.js v20+.

## 4. CRITÉRIOS DE SEVERIDADE APLICADOS

- **ALTA**: Bloqueios de sistema, vulnerabilidades críticas de segurança ou falhas funcionais core.
- **MÉDIA**: Otimização de performance necessária, refatoração de dívida técnica ou novas features planeadas.
- **BAIXA**: Melhorias cosméticas, documentação técnica ou ajustes de experiência de developer.
