# Chirpy

A social media API built with Go, featuring user authentication, chirp management, and premium user upgrades.

## Features

- User authentication with JWT tokens and refresh tokens
- Chirp creation, retrieval, and deletion
- Profanity filtering for chirps
- Premium user upgrades (Chirpy Red)
- Webhook integration for user upgrades
- Admin metrics and development tools

## API Endpoints

### Authentication
- `POST /api/users` - Create a new user
- `POST /api/login` - Login and receive access/refresh tokens
- `POST /api/refresh` - Get a new access token using refresh token
- `POST /api/revoke` - Revoke a refresh token

### Chirps
- `GET /api/chirps` - Get all chirps
  - Optional query parameters:
    - `author_id`: Filter chirps by author ID
    - `sort`: Sort chirps by creation date
      - `asc` (default): Sort in ascending order (oldest first)
      - `desc`: Sort in descending order (newest first)
- `GET /api/chirps/{chirpID}` - Get a specific chirp
- `POST /api/chirps` - Create a new chirp
- `DELETE /api/chirps/{chirpID}` - Delete a chirp (author only)

### User Management
- `PUT /api/users` - Update user email and password
- `POST /api/polka/webhooks` - Handle user upgrade events (requires API key)

### Admin
- `GET /admin/metrics` - View API usage metrics
- `POST /admin/reset` - Reset database (development only)

## Setup

1. Create a PostgreSQL database:
```bash
createdb chirpy
```

2. Set up environment variables in `.env`:
```env
DB_URL=postgres://username:password@localhost:5432/chirpy?sslmode=disable
JWT_SECRET=your-secret-key
POLKA_KEY=your-polka-api-key
PLATFORM=dev  # or prod
```

3. Run database migrations:
```bash
goose -dir sql/schema postgres "postgres://username:password@localhost:5432/chirpy?sslmode=disable" up
```

4. Generate database code:
```bash
sqlc generate
```

5. Start the server:
```bash
go run .
```

## Testing

The server includes a test suite that can be run with:
```bash
bootdev run 1304e939-bf50-48d3-a351-b35faafc267d -s
```

## Development

- The server runs on port 8080 by default
- Development mode enables additional endpoints and features
- Profanity filtering is case-insensitive
- Chirps are limited to 140 characters
- Access tokens expire after 1 hour
- Refresh tokens expire after 60 days

## Security

- Passwords are hashed using bcrypt
- JWT tokens are used for authentication
- Refresh tokens are stored securely in the database
- User upgrades are processed via webhooks
- Admin endpoints are protected in production

## Database Schema

The application uses the following main tables:
- `users` - User accounts and authentication
- `chirps` - User posts
- `refresh_tokens` - Token management 