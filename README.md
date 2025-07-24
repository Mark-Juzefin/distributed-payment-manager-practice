# Payment Manager

**Payment Manager** is a backend service that simulates a merchant system integrated with external payment providers.  
It receives and processes webhook events related to orders and disputes.

TODO
- Наглядно логувати усі операції для репрезентативності
- Особливо у інтеграційних тестах


## Features
- **Order Processing**: Handles incoming events to create and update orders based on external provider data.
- **Dispute Flow**: Simulates a basic chargeback lifecycle:
  - Receives `chargeback.created` events.
  - Stores open disputes.
  - Allows submission of evidence (representment).
  - Processes dispute resolution via `chargeback.closed` events.

## Tech Highlights
- Swappable repository layer (PostgreSQL / MongoDB)
- MongoDB Time-Series collection for storing events
- Sharding-ready design for scaling


## Quick Start

```bash
# Set DB_DRIVER=postgres or mongo
DB_DRIVER=mongo go run cmd/server/main.go
```
