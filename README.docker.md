# Docker Compose Setup

This project includes Docker Compose configuration for running PostgreSQL locally.

## Prerequisites

- Docker and Docker Compose installed
- Environment variables configured (see `.env.example`)

## Quick Start

1. **Start PostgreSQL:**
   ```bash
   docker-compose up -d
   ```

2. **Check PostgreSQL is running:**
   ```bash
   docker-compose ps
   ```

3. **View PostgreSQL logs:**
   ```bash
   docker-compose logs -f postgres
   ```

4. **Stop PostgreSQL:**
   ```bash
   docker-compose down
   ```

5. **Stop and remove volumes (deletes all data):**
   ```bash
   docker-compose down -v
   ```

## Environment Variables

Create a `.env` file based on `.env.example` and configure:

```bash
# PostgreSQL connection (if using PostgreSQL instead of SQLite)
POSTGRES_USER=gemini_user
POSTGRES_PASSWORD=gemini_password
POSTGRES_DB=gemini_db
POSTGRES_PORT=5432
```

## Connecting to PostgreSQL

Once the container is running, you can connect to PostgreSQL:

```bash
# Using psql (if installed locally)
psql -h localhost -p 5432 -U gemini_user -d gemini_db

# Or using Docker
docker-compose exec postgres psql -U gemini_user -d gemini_db
```

## Database Migrations

After starting PostgreSQL, you'll need to run your database migrations. The application should handle this automatically, or you can run them manually using your migration tool.

## Persistent Data

PostgreSQL data is stored in a Docker volume named `postgres_data`. This ensures data persists even if you stop and remove the container.

To backup the database:
```bash
docker-compose exec postgres pg_dump -U gemini_user gemini_db > backup.sql
```

To restore from backup:
```bash
docker-compose exec -T postgres psql -U gemini_user gemini_db < backup.sql
```
