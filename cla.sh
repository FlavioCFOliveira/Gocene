bash#!/bin/bash

# Configura as variáveis e lança o Claude Code com o Ollama remoto
ANTHROPIC_AUTH_TOKEN="ollama" ANTHROPIC_BASE_URL="http://0.0.0.0:11333" claude --model gemma4:31b --dangerously-skip-permissions
