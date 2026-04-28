# Docker Compose Setup

This project includes optional Docker Compose configuration for running PostgreSQL and Redis locally. The default Nix development workflow uses `dev-services start` instead.

## Prerequisites

- Docker and Docker Compose installed
- Environment variables configured (see `.env.example`)

## Quick Start

1. **Start PostgreSQL and Redis:**
   ```bash
   docker-compose up -d
   ```

2. **Check services are running:**
   ```bash
   docker-compose ps
   ```

3. **View service logs:**
   ```bash
   docker-compose logs -f
   ```

4. **Stop services:**
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
# PostgreSQL connection
POSTGRES_USER=postgres
POSTGRES_PASSWORD=password
POSTGRES_DB=gemini_db
POSTGRES_PORT=5432
```

## Connecting to PostgreSQL

Once the container is running, you can connect to PostgreSQL:

```bash
# Using psql (if installed locally)
psql -h localhost -p 5432 -U postgres -d gemini_db

# Or using Docker
docker-compose exec db psql -U postgres -d gemini_db
```

## Database Migrations

After starting PostgreSQL, you'll need to run your database migrations. The application should handle this automatically, or you can run them manually using your migration tool.

## Persistent Data

PostgreSQL data is stored in the Docker volume named `pg-data`, and Redis data is stored in `redis-data`. This keeps data around if you stop and remove the containers without deleting volumes.

To backup the database:
```bash
docker-compose exec db pg_dump -U postgres gemini_db > backup.sql
```

To restore from backup:
```bash
docker-compose exec -T db psql -U postgres gemini_db < backup.sql
```
