# Feature: Silvergate Transactions Deep Dive

**Status:** Planned

## Overview

Поглиблена практика з PostgreSQL транзакціями всередині Silvergate.
Зараз операції — це одно-рядкові апдейти. Хочеться додати реалістичність:
balances, double-entry ledger, multi-row атомарність, deadlock-сценарії,
порівняння isolation levels під concurrent навантаженням.

Це навчальна "пісочниця" для transactions/concurrency. Конкретні теми і
сабтаски набираємо коли почнемо. Можливі напрямки: balances + двозаписова
атомарність, double-entry ledger з CHECK на нуль-суму, ізоляційні рівні,
load test для провокування serialization errors / deadlock-ів.

## Notes
- Created: 2026-04-17
- Продовження Feature 007 (Payment System Logic, Phase 1-2) — там були
  основи single-row safety, тут — multi-row
