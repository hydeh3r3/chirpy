# Chirpy

A social media API built with Go that allows users to create and validate "chirps" (posts limited to 140 characters).

## Features

- User management with PostgreSQL database
- Chirp validation and profanity filtering
- Admin dashboard with request metrics
- Health check endpoint
- Development mode with reset functionality

## Prerequisites

- Go 1.23.4 or higher
- PostgreSQL
- SQLC
- Goose (for database migrations)

## Setup

1. Clone the repository
2. Create a `.env` file with the following variables:
   ```
   DB_URL="postgres://username:password@localhost:5432/chirpy?sslmode=disable"
   PLATFORM="dev"  # Set to "prod" in production
   ```

3. Create the database:
   ```bash
   createdb chirpy
   ```

4. Run database migrations:
   ```bash
   goose -dir sql/schema postgres "postgres://username:password@localhost:5432/chirpy?sslmode=disable" up
   ```

5. Generate SQLC code:
   ```bash
   sqlc generate
   ```

6. Run the server:
   ```bash
   go run main.go
   ```

## API Endpoints

### Public Endpoints

- `GET /api/healthz` - Health check endpoint
- `POST /api/validate_chirp` - Validate and clean chirp content
- `POST /api/users` - Create a new user

### Admin Endpoints

- `GET /admin/metrics` - View request metrics dashboard
- `POST /admin/reset` - Reset metrics and database (dev mode only)

### File Server

- `GET /app/*` - Serve static files

## Development

The project uses:
- SQLC for type-safe database queries
- Goose for database migrations
- PostgreSQL for data storage

## License

MIT 