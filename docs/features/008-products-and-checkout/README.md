# Feature: Silvergate Products

**Status:** In Progress

## Overview

Розширити Silvergate доменом продуктів. Зараз сервіс знає тільки про
низькорівневі операції з картою (auth/capture/void/refund) — треба додати
поняття "що саме купують".

**Що додаємо:**
- CRUD для продуктів (entity, repo, міграція, HTTP handlers)
- Один високорівневий endpoint типу `POST /purchase` — приймає
  `{merchant_id, order_id, product_id, card_token}`, всередині дістає
  продукт з БД, викликає вже існуючу логіку auth+capture, повертає
  `transaction_id`

Існуючі low-level endpoints стають внутрішніми будівельними блоками,
`/purchase` тільки їх композує.

## Notes
- Created: 2026-04-17
- Деталі (схема таблиці, контракти DTO, статуси продукту) — у
  `plan-subtask-N.md` коли почнемо конкретну підзадачу
