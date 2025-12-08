# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Automated cryptocurrency trading application written in Go that executes buy/sell strategies on bitflyer and OKEX exchanges. The application uses scheduled jobs to place buy orders at specific times based on trading strategies, monitors order fulfillment, and automatically places sell orders at higher prices (+1.5% or +3% depending on strategy).

## Build and Run

### Building the Application

```bash
# Simple build
make build

# Run directly without building binary
make run

# Run binary
make run-binary

# Build for Amazon Linux deployment
GOOS=linux GOARCH=amd64 go build -o crypto-trading ./go/cmds/bifflyer_trading/main.go
```

### Configuration Requirements

Before running, you must set up configuration files in the `go/` directory:

1. `config.ini` - Public configuration (already in repo)
2. `private_config.ini` - Sensitive configuration (copy from `[sample]private_config.ini` and remove `[sample]` prefix)
   - Contains API keys, database credentials, and Slack webhook URL
   - **Never commit this file to version control**

### Database Setup

```bash
cd db
make up  # Installs Atlas migration tool and applies all migrations
```

## Architecture

### Entry Point and Initialization

The application starts in `go/cmds/bifflyer_trading/main.go`:
1. Loads configuration from `config.ini` and `private_config.ini` (go/config/config.go:16)
2. Initializes MySQL database connection (go/models/mysqlBase.go)
3. Sets up logging
4. Starts the trading service (go/app/bitflyerApp/service.go:84)

### Job Scheduling System

The core service (`go/app/bitflyerApp/service.go`) uses the `carlescere/scheduler` package to run periodic jobs:

**Buy Order Jobs** - Scheduled at 6:30 AM JST daily:
- Multiple strategies for BTC and ETH with different weekday conditions
- Each strategy has a different pricing calculation (e.g., `Stg0BtcLtp3low7`, `Stg1BtcLtp997`)

**Monitoring Jobs** - Run continuously:
- Order synchronization (every 90 seconds) - syncs buy/sell orders with exchange APIs
- Filled order check (every 90 seconds) - checks if orders have been executed
- Sell order placement (every 180 seconds) - places sell orders for filled buy orders
- Strange record deletion (every 7200 seconds) - removes corrupted records

**Daily Jobs**:
- Price history recording (6:00 AM and 6:00 PM) - saves BTC/ETH price snapshots
- Daily profit/loss report (6:45 AM) - sends summary to Slack
- Order cancellation (11:45 PM) - cancels expired unfilled orders

**Graceful Shutdown**:
- Scheduled at 12:20 PM and 10:50 PM JST
- Waits for running jobs to complete (max 5 minutes timeout)
- Prevents new jobs from starting during shutdown
- Waits 15 additional minutes before exit to avoid RDS schedule conflicts

### Job Separation Pattern

Job functions are separated into individual files for maintainability:
- `placeBuyOrder.go` - Buy order placement logic
- `placeSellOrder.go` - Sell order placement logic
- `savePriceHistoryJob.go` - Price history recording
- `sendResultJob.go` - Daily profit/loss reporting
- `filledCheckJob.go` - Order fulfillment checking
- `syncBuyOrders.go` - Order synchronization with exchange

Each job file contains the job-specific logic and is invoked from wrapper functions in `service.go`.

### Trading Strategy System

Strategies are defined in `go/enums/strategy.go` as integer constants:

**BTC Strategies** (0-3):
- `Stg0BtcLtp3low7`, `Stg1BtcLtp997`, `Stg2BtcLtp98`, `Stg3BtcLtp90`

**ETH Strategies** (10-14):
- `Stg10EthLtp995`, `Stg11EthLtp98`, `Stg12EthLtp97`, `Stg13EthLtp3low7`, `Stg14EthLtp90`

Each strategy determines:
- The buy price calculation method
- The sell price markup (1.5% or 3%)
- The order expiration time (1440 or 3600 minutes)

Strategy-specific sell prices are calculated in `go/models/events.go:170`:
- Strategies 3 and 14 use +3% markup
- All other strategies use +1.5% markup

### Database Layer

**Connection Management** (`go/models/mysqlBase.go`):
- Initializes both standard `database/sql` connection (`AppDB`) and GORM connection (`GormDB`)
- Connection string loaded from `private_config.ini`

**Main Tables**:
- `buy_orders` - Buy order records with status tracking
  - Status values: `UNFILLED`, `FILLED`, `FILLED(SELL ORDER PLACED)`, `CANCELLED`
- `sell_orders` - Sell order records linked to buy orders via `parentid`
- `price_histories` - Historical price snapshots with 24-hour ratio calculations

**Order Status Flow**:
1. Buy order placed → `UNFILLED`
2. Order filled → `FILLED`
3. Sell order placed → `FILLED(SELL ORDER PLACED)`
4. Sell order filled → Profit recorded
5. Expired orders → `CANCELLED`

### Exchange Integration

**bitflyer** (`go/bitflyer/bitflyer.go`):
- Primary exchange for BTC_JPY and ETH_JPY trading
- API client handles order placement, cancellation, and balance checks

**OKEX** (`go/okex/okex.go`):
- Secondary exchange support (currently less actively used)

### Slack Integration

Error notifications and daily reports are sent via Slack webhook (`go/slack/slack.go`):
- Order placement success/failure
- API errors with detailed context
- Daily profit/loss summaries
- Cancellation notifications

## Database Migrations

### Migration Workflow

All migration commands must be run from the `db/` directory:

```bash
cd db

# Check migration status
make atlas-status

# Apply pending migrations
make atlas-apply

# Create new migration (after updating schema.hcl)
make atlas-diff n=20250113000005_your_migration_name

# Update checksums after manual migration file edits
make atlas-hash

# Reset database (⚠️ DESTRUCTIVE - deletes all data)
make reset-db
```

### Creating Migrations

1. Edit `db/crypto-trading-db/atlas/schema.hcl` to reflect schema changes
2. Run `make atlas-diff n=<timestamp>_<description>` to generate migration SQL
3. Review the generated SQL file in `db/crypto-trading-db/atlas/migrations/`
4. Apply with `make atlas-apply`

Database connection settings are in `db/envs/.db.env` (passwords are automatically URL-encoded).

## Infrastructure (AWS + Terraform)

### Terraform Commands

All Terraform commands use the Makefile wrapper (never use `terraform` directly):

```bash
# Show available commands
make help

# Format code
make fmt

# Initialize Terraform
make init

# Plan changes
make plan

# Apply changes
make apply

# Destroy infrastructure
make destroy
```

### AWS Resources

**RDS MySQL**:
- Version 8.4.7
- Automatically started/stopped via EventBridge:
  - Start: 3:00 AM JST (18:00 UTC previous day) and 4:30 PM JST (7:30 UTC)
  - Stop: 12:30 PM JST (3:30 UTC) and 11:00 PM JST (14:00 UTC)

**Terraform State**:
- Backend: S3 bucket `tfstate-crypto-trading-20251113`
- AWS Profile: `crypto-trading-20251113`
- Required version: exactly `1.13.5`

**Important**: `terraform.tfvars` contains sensitive data and must never be committed.

## Development Rules

### Timezone Handling

- Scheduler times may depend on system timezone
- For JST-specific behavior, explicitly check time within job functions
- RDS schedule uses UTC (JST = UTC + 9 hours)
- Application graceful shutdown at 12:20 and 22:50 JST coordinates with RDS stop times

### Error Handling Requirements

- All API errors must be sent to Slack with context
- Include order details in error messages: OrderID, Price, Size, Strategy
- Capture and log `Status != 0` or `ErrorMessage` fields from API responses
- Check API response status before assuming success

### Code Organization

- Separate job functions into individual files (e.g., `savePriceHistoryJob.go`, `sendResultJob.go`)
- Use DRY principle - extract common logic into utility functions
- Wrap scheduled jobs with `wrapJob()` to support graceful shutdown tracking

### Configuration Files

- `go/config.ini` - Public settings (committed)
- `go/private_config.ini` - Secrets (never committed, in `.gitignore`)
- `terraform.tfvars` - Infrastructure secrets (never committed, in `.gitignore`)
- `db/envs/.db.env` - Database credentials (never committed)

### VS Code Debugging

A launch configuration exists at `.vscode/launch.json` with working directory set to `go/` to ensure configuration files are found correctly.

## Key Constants and Configuration

- **Budget Criteria**: Minimum JPY balance required to place buy orders (default: 500,000)
- **Cancel Criteria**: Orders older than this are automatically cancelled (defined in utils)
- **Max Buy/Sell Orders**: Configurable limits prevent over-trading
- **Sell Price Markup**: +1.5% (standard) or +3% (high-risk strategies)
- **Order Expiration**: 1440 minutes (1 day) or 3600 minutes (2.5 days) depending on strategy

## Important Notes

- The application uses `runtime.Goexit()` at the end of `StartBfService()` to keep the scheduler running
- Job wrapper (`wrapJob()`) tracks running jobs with `sync.WaitGroup` for graceful shutdown
- Shutdown flag prevents new job execution while waiting for existing jobs
- Database uses both `database/sql` (`AppDB`) and GORM (`GormDB`) connections
