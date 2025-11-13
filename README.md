# Crypto Trading Golang

Automated cryptocurrency trading web application implemented in GoLang.

This application places buy orders at specific times in a day, checks if they're filled, and automatically places sell orders at a slightly higher price (currently +1.5% is hard coded).

## Supported Currencies & Exchanges

1. **bitflyer** (BTC, ETH)
2. **OKEX** (BTC, ETH, BCH, EOS, BSV, OKB)

※ Spot Trading only. Margin or FX trading are not supported.

## Features

### Trading Features
- Automated buy/sell order placement
- Order status tracking (UNFILLED, FILLED, FILLED(SELL ORDER PLACED), CANCELLED)
- Multiple trading strategies with configurable parameters
- Weekday-based order scheduling
- Automatic order cancellation for expired orders
- Order synchronization with exchange APIs

### Price History Tracking
- Automatic price history recording at 6:00 AM and 6:00 PM daily
- 24-hour price ratio calculation (e.g., 0.95 = 95%, 1.21 = 121%)
- Historical price data storage for BTC_JPY and ETH_JPY

### Risk Management
- JPY balance checking before placing buy orders
- Configurable budget criteria to prevent trading when balance is low
- Maximum order limits (buy/sell) to prevent over-trading

### Monitoring & Notifications
- Slack integration for order notifications and error alerts
- Daily profit/loss reports sent to Slack
- Detailed error messages including API error responses

### Infrastructure
- AWS RDS MySQL 8.4.7 database
- Automated RDS start/stop scheduling via EventBridge
- Database migration management with Atlas

## Database Schema

The application uses MySQL 8.4.7 with the following main tables:

- **buy_orders**: Stores buy order information
  - Status values: `UNFILLED`, `FILLED`, `FILLED(SELL ORDER PLACED)`, `CANCELLED`
- **sell_orders**: Stores sell order information
  - Status values: `UNFILLED`, `FILLED`, `CANCELLED`
- **price_histories**: Stores historical price data
  - Tracks price changes and 24-hour price ratios

## Database Migrations

This project uses [Atlas](https://atlasgo.io/) for database schema migrations.

### Directory Structure

```
db/
├── Makefile                    # Migration commands
├── envs/
│   └── .db.env                # Database connection settings
└── crypto-trading-db/
    └── atlas/
        ├── atlas.hcl          # Atlas configuration
        ├── schema.hcl          # Current schema definition
        └── migrations/         # Migration SQL files
            ├── 20250101000000_create_buy_and_sell_orders.sql
            ├── 20250113000000_remove_filled_column.sql
            ├── 20250113000001_add_remarks_column.sql
            ├── 20250113000002_update_buy_orders_status_comment.sql
            └── 20250113000004_create_price_history.sql
```

### Migration Commands

All migration commands should be run from the `db/` directory:

```bash
cd db

# Install Atlas migration tool
make install-migration-tool

# Check migration status
make atlas-status

# Apply pending migrations
make atlas-apply

# Create a new migration file (after updating schema.hcl)
make atlas-diff n=20250113000005_your_migration_name

# Update migration checksums
make atlas-hash

# Reset database (⚠️ WARNING: This will delete all data)
make reset-db
```

### Migration Workflow

1. **Update schema**: Modify `db/crypto-trading-db/atlas/schema.hcl` to reflect desired schema changes
2. **Generate migration**: Run `make atlas-diff n=<timestamp>_<description>` to create a new migration file
3. **Review migration**: Check the generated SQL file in `migrations/` directory
4. **Apply migration**: Run `make atlas-apply` to apply changes to the database

### Database Configuration

Database connection settings are configured in `db/envs/.db.env`:

```bash
DB_HOST=your-rds-endpoint
DB_PORT=3306
DB_NAME=crypto_trading_db
DB_USER=your_username
DB_PASSWORD=your_password
```

**Note**: The password is automatically URL-encoded (special characters like `&` are converted to `%26`) for use in connection strings.

## How to Build

### Simple Build
```bash
go build -o crypto-trading ./go/cmds/bifflyer_trading/main.go
```

### Build for Amazon Linux
```bash
GOOS=linux GOARCH=amd64 go build -o crypto-trading ./go/cmds/bifflyer_trading/main.go
```

## Configuration

### Configuration Files

1. **`go/config.ini`**: Application settings (public configuration)
2. **`go/private_config.ini`**: Sensitive settings (API keys, database credentials)
   - Copy from `[sample]private_config.ini` and remove `[sample]` prefix
   - **DO NOT commit this file to version control**

### Key Configuration Parameters

- `budget_criteria`: Minimum JPY balance required to place buy orders (default: 500000)
- `max_buy_orders`: Maximum number of concurrent buy orders
- `max_sell_orders`: Maximum number of concurrent sell orders
- Trading amounts for BTC and ETH

## How to Run

1. **Set up database**:
   ```bash
   cd db
   make up  # Installs Atlas and applies migrations
   ```

2. **Configure settings**:
   - Copy `[sample]private_config.ini` to `private_config.ini`
   - Fill in API keys, database credentials, and Slack webhook URL
   - Adjust settings in `config.ini` as needed

3. **Build the application**:
   ```bash
   go build -o crypto-trading ./go/cmds/bifflyer_trading/main.go
   ```

4. **Run the application**:
   ```bash
   ./crypto-trading
   ```

## Scheduled Jobs

The application runs various scheduled jobs:

- **Buy Orders**: Scheduled at specific times based on weekday and strategy
- **Sell Orders**: Checks for filled buy orders every 180 seconds
- **Order Synchronization**: Syncs orders with exchange APIs every 90 seconds
- **Filled Order Check**: Checks order status every 90 seconds
- **Price History**: Records BTC and ETH prices at 6:00 AM and 6:00 PM daily
- **Daily Reports**: Sends profit/loss summary to Slack at 6:45 AM daily
- **Order Cancellation**: Cancels expired orders at 11:45 PM daily

## AWS Infrastructure

This project uses Terraform to manage AWS infrastructure.

### Directory Structure

```
terraform/
├── main.tf                 # Main Terraform configuration
├── terraform.tfvars        # Variable values (contains sensitive data)
└── modules/
    ├── network/            # VPC, Subnets, Route Tables
    ├── database/          # RDS MySQL instance
    └── scheduler/         # EventBridge scheduler for RDS start/stop
```

### RDS Scheduling

The RDS instance is automatically started and stopped via EventBridge:

- **Start**: 3:00 AM JST (18:00 UTC previous day) and 4:30 PM JST (7:30 UTC)
- **Stop**: 12:30 PM JST (3:30 UTC) and 11:00 PM JST (14:00 UTC)

This ensures the database is only running during trading hours (3:00 AM - 12:30 PM and 4:30 PM - 11:00 PM JST).

### Terraform Commands

All Terraform commands should be run from the project root directory:

```bash
# Show all available commands
make help

# Format Terraform code
make fmt

# Initialize Terraform
make init

# Plan infrastructure changes
make plan

# Apply infrastructure changes
make apply
```

### Important Notes

1. **Sensitive Data**: The `terraform.tfvars` file contains sensitive information
   - **DO NOT commit this file to version control**
   - It is already in `.gitignore`

2. **Terraform State**: Stored in S3 backend (`tfstate-crypto-trading-20251113`)

3. **AWS Profile**: Uses profile `crypto-trading-20251113`

4. **Terraform Version**: Requires exactly version `1.13.5`

## Development

### VS Code Debugging

The project includes a VS Code launch configuration (`.vscode/launch.json`) for debugging. The working directory is set to `go/` to ensure configuration files are found correctly.

### Code Structure

```
go/
├── app/
│   └── bitflyerApp/        # bitflyer trading logic
│       ├── service.go      # Main service and job scheduling
│       ├── placeBuyOrder.go
│       ├── placeSellOrder.go
│       ├── savePriceHistoryJob.go
│       └── sendResultJob.go
├── models/                 # Database models and operations
│   ├── events.go
│   ├── price_history.go
│   └── mysqlBase.go
├── bitflyer/               # bitflyer API client
├── config/                 # Configuration management
└── cmds/
    └── bifflyer_trading/
        └── main.go         # Application entry point
```

## Error Handling

The application includes comprehensive error handling:

- API errors are captured and sent to Slack with detailed messages
- Database errors are logged and reported
- Order placement failures include context (order ID, price, size, strategy)
- Insufficient funds errors are specifically handled and reported

## License

[Add your license information here]
