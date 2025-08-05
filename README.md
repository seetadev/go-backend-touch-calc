# Tornado-Nginx Go Backend

A Go-based conversion of the Python Tornado backend for the Aspiring Investments platform. This backend provides cloud-based spreadsheet and business application services with AWS S3 storage integration.

## Features

- **Authentication System**: User registration, login, password reset with secure cookie-based sessions
- **Cloud Storage**: AWS S3-based file and directory management system
- **Web Applications**: Spreadsheet applications with real-time collaboration features
- **Email Services**: Amazon SES integration for transactional emails
- **Dropbox Integration**: OAuth-based Dropbox sync functionality
- **Session Management**: In-memory session management with automatic cleanup
- **RESTful API**: Clean REST endpoints for all services

## Architecture

```
go-backend/
├── cmd/server/          # Application entry point
├── internal/
│   ├── auth/           # Authentication service
│   ├── config/         # Configuration management
│   ├── email/          # Email service (SES)
│   ├── handlers/       # HTTP request handlers
│   ├── models/         # Data models
│   ├── session/        # Session management
│   └── storage/        # Storage interface and S3 implementation
├── pkg/
│   ├── middleware/     # HTTP middleware
│   └── utils/          # Utility functions
└── web/
    ├── static/         # Static assets
    └── templates/      # HTML templates
```

## Technology Stack

- **Language**: Go 1.21+
- **Web Framework**: Gin
- **Cloud Storage**: AWS S3
- **Email Service**: Amazon SES
- **Session Store**: In-memory cache with TTL
- **Authentication**: bcrypt password hashing
- **Containerization**: Docker & Docker Compose
- **Reverse Proxy**: Nginx

## Installation & Setup

### Prerequisites

- Go 1.21 or higher
- Docker and Docker Compose
- AWS account with S3 and SES configured
- Nginx (for production deployment)

### Environment Configuration

1. Copy the example environment file:
   ```bash
   cp .env.example .env
   ```

2. Update the `.env` file with your AWS credentials and configuration:
   ```env
   AWS_ACCESS_KEY_ID=your_aws_access_key
   AWS_SECRET_ACCESS_KEY=your_aws_secret_key
   AWS_REGION=us-east-1
   S3_BUCKET=your-s3-bucket-name
   FROM_EMAIL=your-verified-ses-email@domain.com
   ```

### Development Setup

1. **Install dependencies**:
   ```bash
   go mod download
   ```

2. **Run the application**:
   ```bash
   go run cmd/server/main.go
   ```

3. **Access the application**:
   - Server: http://localhost:8080
   - Health check: http://localhost:8080/health

### Docker Deployment

1. **Build and run with Docker Compose**:
   ```bash
   docker-compose up --build
   ```

2. **Access the application**:
   - Application: http://localhost
   - Backend (direct): http://localhost:8080

### Production Deployment

For production deployment, use the included nginx configuration with proper SSL certificates and security headers.

## API Endpoints

### Authentication
- `POST /iauth` - Multi-purpose authentication (login/register/logout)
- `POST /login` - User login
- `POST /register` - User registration
- `POST /logout` - User logout
- `GET /pwreset` - Password reset form
- `POST /pwreset` - Process password reset

### Web Applications
- `POST /iwebapp` - Web application operations (save/load/list files)
- `GET /browser/:app/:code/:file` - Access web applications
- `GET /browser` - Landing page

### Email
- `POST /irunasemailer` - Send emails via SES

### Dropbox Integration
- `GET /browser/:app/dropbox` - Dropbox OAuth operations
- `POST /browser/:app/dropbox` - Dropbox file operations

### System
- `GET /health` - Health check endpoint

## Key Components

### Authentication Service
- Secure password hashing with bcrypt
- Session-based authentication
- Password reset with secure tokens
- User management with AWS S3 storage

### Storage Service
- AWS S3-based file system abstraction
- Directory and file operations
- JSON-based metadata storage
- Hierarchical path structure

### Session Management
- In-memory session storage with TTL
- Automatic cleanup of expired sessions
- Session-based user state management

### Email Service
- Amazon SES integration
- HTML and text email support
- Transactional email templates

## Configuration

The application uses environment variables for configuration:

| Variable | Description | Default |
|----------|-------------|---------|
| `ENVIRONMENT` | Environment (development/production) | development |
| `PORT` | Server port | 8080 |
| `AWS_ACCESS_KEY_ID` | AWS access key | - |
| `AWS_SECRET_ACCESS_KEY` | AWS secret key | - |
| `AWS_REGION` | AWS region | us-east-1 |
| `S3_BUCKET` | S3 bucket name | aspiring-cloud-storage |
| `FROM_EMAIL` | SES verified sender email | - |

## Security Features

- Secure cookie-based sessions
- Password hashing with bcrypt
- CORS protection
- Rate limiting (via nginx)
- Security headers
- Input validation
- SQL injection prevention (no SQL used)

## Performance Optimizations

- Connection pooling for AWS services
- In-memory session caching
- Static file serving via nginx
- Gzip compression
- HTTP/2 support (with proper nginx config)

## Monitoring & Health Checks

- Health check endpoint at `/health`
- Docker health checks configured
- Nginx upstream health monitoring
- Structured logging

## Development

### Project Structure

The project follows Go standard project layout:
- `cmd/` - Application entry points
- `internal/` - Private application code
- `pkg/` - Public library code that can be used by external applications
- `web/` - Web application specific components

### Adding New Features

1. Define models in `internal/models/`
2. Implement business logic in appropriate service packages
3. Create handlers in `internal/handlers/`
4. Add routes in `cmd/server/main.go`
5. Add tests in corresponding `_test.go` files

### Testing

Run tests with:
```bash
go test ./...
```

## Migration from Python

This Go backend is a direct conversion of the original Python Tornado backend with the following improvements:

- **Performance**: Significantly faster due to Go's compiled nature and goroutines
- **Memory Efficiency**: Lower memory footprint compared to Python
- **Concurrency**: Better handling of concurrent requests
- **Type Safety**: Compile-time type checking prevents runtime errors
- **Deployment**: Single binary deployment, no dependency management issues

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests for new functionality
5. Submit a pull request

## License

This project maintains the same license as the original Python implementation.